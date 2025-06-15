package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileChange represents a single file to be modified.
type FileChange struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// MdiffJSON is the top-level structure for the JSON input.
type MdiffJSON struct {
	Changes []FileChange `json:"changes"`
}

// IgnoreMatcher holds gitignore patterns and logic.
type IgnoreMatcher struct {
	patterns         []string
	gitignoreRootAbs string // Absolute path to the directory containing the .gitignore file
}

// NewIgnoreMatcher creates a new IgnoreMatcher.
// customGitignorePath is the user-provided path to a .gitignore file (can be empty).
// scanDirAbs is the absolute path to the root directory being scanned.
func NewIgnoreMatcher(customGitignorePath, scanDirAbs string) (*IgnoreMatcher, error) {
	effectiveGitignorePath := customGitignorePath
	if effectiveGitignorePath == "" {
		effectiveGitignorePath = filepath.Join(scanDirAbs, ".gitignore")
	} else {
		absPath, err := filepath.Abs(effectiveGitignorePath)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for custom gitignore '%s': %w", effectiveGitignorePath, err)
		}
		effectiveGitignorePath = absPath
	}

	matcher := &IgnoreMatcher{
		patterns:         []string{},
		gitignoreRootAbs: filepath.Dir(effectiveGitignorePath),
	}

	fileInfo, err := os.Stat(effectiveGitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return matcher, nil
		}
		return nil, fmt.Errorf("failed to stat gitignore file '%s': %w", effectiveGitignorePath, err)
	}

	if fileInfo.IsDir() {
		return nil, fmt.Errorf("gitignore path '%s' is a directory, not a file", effectiveGitignorePath)
	}

	file, err := os.Open(effectiveGitignorePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open gitignore file '%s': %w", effectiveGitignorePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "!") {
			continue
		}
		matcher.patterns = append(matcher.patterns, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read gitignore file '%s': %w", effectiveGitignorePath, err)
	}

	return matcher, nil
}

// IsIgnored checks if a given path should be ignored based on the loaded patterns.
// absItemPath is the absolute path to the item (file or directory).
// itemIsDir indicates if the item is a directory.
func (m *IgnoreMatcher) IsIgnored(absItemPath string, itemIsDir bool) (bool, error) {
	if len(m.patterns) == 0 {
		return false, nil
	}

	pathRelToGitignoreRoot, err := filepath.Rel(m.gitignoreRootAbs, absItemPath)
	if err != nil {
		return false, nil
	}
	pathRelToGitignoreRoot = filepath.ToSlash(pathRelToGitignoreRoot)

	for _, rawPattern := range m.patterns {
		pattern := rawPattern
		isDirOnlyPattern := strings.HasSuffix(pattern, "/")
		pattern = strings.TrimSuffix(pattern, "/")

		if isDirOnlyPattern && !itemIsDir {
			continue
		}

		cleanPattern := filepath.ToSlash(pattern)
		var matched bool
		var matchErr error

		// Handle patterns anchored to the root of the .gitignore directory
		if strings.HasPrefix(rawPattern, "/") {
			actualPatternToMatch := strings.TrimPrefix(cleanPattern, "/")
			matched, matchErr = filepath.Match(actualPatternToMatch, pathRelToGitignoreRoot)
		} else if strings.Contains(cleanPattern, "/") {
			// Pattern contains a directory separator, match against the full relative path
			matched, matchErr = filepath.Match(cleanPattern, pathRelToGitignoreRoot)
		} else {
			// Pattern does not contain a directory separator, match against any path component
			matched, matchErr = filepath.Match(cleanPattern, filepath.Base(pathRelToGitignoreRoot))
		}

		if matchErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: malformed gitignore pattern '%s' (processed as '%s'): %v\n", rawPattern, cleanPattern, matchErr)
			continue
		}

		if matched {
			return true, nil
		}
	}
	return false, nil
}

// extractFileContent extracts content from files in a directory based on extensions.
// scanDirAbs must be an absolute path to the directory to scan.
func extractFileContent(scanDirAbs string, extensions []string, ignoreMatcher *IgnoreMatcher) (string, error) {
	var allContent strings.Builder

	err := filepath.Walk(scanDirAbs, func(currentPathAbs string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: error accessing path %s: %v. Skipping.\n", currentPathAbs, err)
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil // Skip this file/dir entry, continue walk
		}

		if ignoreMatcher != nil {
			isIgnored, ignoreErr := ignoreMatcher.IsIgnored(currentPathAbs, info.IsDir())
			if ignoreErr != nil {
				// Don't fail the whole walk, just log it and potentially skip.
				// Depending on desired strictness, could return ignoreErr.
				fmt.Fprintf(os.Stderr, "Warning: error checking ignore status for %s: %v. Proceeding without ignore check for this item.\n", currentPathAbs, ignoreErr)
			} else if isIgnored {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil // Ignored file
			}
		}

		if info.IsDir() {
			// If it's the root directory itself, don't skip, just proceed.
			if currentPathAbs == scanDirAbs {
				return nil
			}
			// Add specific directory names to ignore if needed, e.g. ".git", "node_modules"
			// This is better handled by .gitignore patterns, but as a fallback:
			return nil // Regular directory, continue walking
		}

		// File processing
		ext := filepath.Ext(currentPathAbs)
		foundExt := false
		for _, targetExt := range extensions {
			if ext == targetExt {
				foundExt = true
				break
			}
		}

		if foundExt {
			content, readErr := os.ReadFile(currentPathAbs)
			if readErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to read file %s: %v. Skipping.\n", currentPathAbs, readErr)
				return nil // Skip this file, continue walk
			}

			relPath, relErr := filepath.Rel(scanDirAbs, currentPathAbs)
			if relErr != nil {
				// This should ideally not happen if currentPathAbs is under scanDirAbs.
				fmt.Fprintf(os.Stderr, "Warning: failed to get relative path for %s (base %s): %v. Using absolute path.\n", currentPathAbs, scanDirAbs, relErr)
				relPath = currentPathAbs // Fallback to absolute path
			}

			allContent.WriteString(fmt.Sprintf("\n<file_path>%s</file_path>\n", filepath.ToSlash(relPath)))
			allContent.Write(content)
			allContent.WriteString(fmt.Sprintf("\n<file_path_end>%s</file_path_end>\n", filepath.ToSlash(relPath)))
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error during directory walk: %w", err)
	}

	return allContent.String(), nil
}

// writeInPlace safely writes content to a file by using a temporary file
// and an atomic rename operation. It also preserves original file permissions.
func writeInPlace(filePath string, content []byte) error {
	info, err := os.Stat(filePath)
	var originalMode os.FileMode = 0644 // Default permissions if file doesn't exist
	if err == nil {
		originalMode = info.Mode()
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("could not stat target file path '%s': %w", filePath, err)
	}
	// If file does not exist, os.Stat returns an error. We proceed to create it.

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil { // 0755 for directories
			return fmt.Errorf("could not create directory %s: %w", dir, err)
		}
	}

	tempFile, err := os.CreateTemp(filepath.Dir(filePath), filepath.Base(filePath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("could not create temporary file in %s: %w", filepath.Dir(filePath), err)
	}
	// Defer removal in case of errors before rename
	defer func() {
		if tempFile != nil { // Check if tempFile was successfully created
			// If rename fails, or an error occurs after creation but before successful rename
			_, statErr := os.Stat(tempFile.Name())
			if statErr == nil { // if temp file still exists
				os.Remove(tempFile.Name())
			}
		}
	}()

	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close() // Close before attempting remove
		return fmt.Errorf("could not write to temporary file '%s': %w", tempFile.Name(), err)
	}

	if err := tempFile.Chmod(originalMode); err != nil {
		tempFile.Close()
		return fmt.Errorf("could not set permissions on temporary file '%s': %w", tempFile.Name(), err)
	}

	if err := tempFile.Close(); err != nil { // Close before rename
		return fmt.Errorf("could not close temporary file '%s': %w", tempFile.Name(), err)
	}

	if err := os.Rename(tempFile.Name(), filePath); err != nil {
		return fmt.Errorf("could not rename temporary file '%s' to '%s': %w", tempFile.Name(), filePath, err)
	}

	tempFile = nil // Indicate successful rename, so defer doesn't try to remove it.
	return nil
}

func printMainUsage() {
	fmt.Println(`
Usage:
  copilot <command> [options] <args...>

Commands:
  apply        Apply changes from a JSON file to target files.
  extract      Extract content from files in a directory based on extensions.

Run 'copilot <command> --help' for more information on a specific command.
`)
}

func printApplyUsage(fs *flag.FlagSet) {
	fmt.Println(`
Usage:
  copilot apply <json_file>

Apply file content changes from a JSON file.
The JSON file should contain an object with a "changes" array,
where each element specifies a "file_path" and its new "content".
Each specified file will be overwritten with the content from the JSON file.
Parent directories for the files will be created if they don't exist.
Paths in the JSON file are typically relative to the current working directory.

Arguments:
  <json_file>       Path to the JSON file containing file content changes.

Example:
  copilot apply ./changes.json
`)
}

func printExtractUsage(fs *flag.FlagSet) {
	fmt.Println(`
Usage:
  copilot extract [extract_options] <directory_path> <file_extensions>

Extract content from files in a directory based on extensions.
Respects .gitignore rules found in <directory_path> or specified via --gitignore.
Outputs a structured format containing file paths and their content.

Arguments:
  <directory_path>     Path to the directory to scan.
  <file_extensions>    Comma-separated list of file extensions (e.g., .js,.ts,.md).

Options:`)
	fs.PrintDefaults()
	fmt.Println(`
Examples:
  copilot extract ./src .js,.ts,.json > extracted_content.txt
  copilot extract --gitignore ./.custom_ignore ./project .go,.java > context.txt
`)
}

func main() {
	if len(os.Args) < 2 || os.Args[1] == "--help" || os.Args[1] == "-h" {
		printMainUsage()
		os.Exit(0)
	}

	command := os.Args[1]

	switch command {
	case "apply":
		applyCmd := flag.NewFlagSet("apply", flag.ExitOnError)
		applyCmd.Usage = func() { printApplyUsage(applyCmd) }

		err := applyCmd.Parse(os.Args[2:])
		if err != nil {
			os.Exit(1)
		}

		if applyCmd.NArg() != 1 {
			fmt.Fprintln(os.Stderr, "Error: Missing <json_file> argument for apply command.")
			applyCmd.Usage()
			os.Exit(1)
		}
		jsonFilePath := applyCmd.Arg(0)

		jsonFileBytes, err := os.ReadFile(jsonFilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading JSON file '%s': %v\n", jsonFilePath, err)
			os.Exit(1)
		}

		var mdiffData MdiffJSON
		err = json.Unmarshal(jsonFileBytes, &mdiffData)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing JSON from file '%s': %v\n", jsonFilePath, err)
			os.Exit(1)
		}

		if len(mdiffData.Changes) == 0 {
			fmt.Fprintln(os.Stderr, "Warning: No changes found in the JSON file.")
			os.Exit(0)
		}

		filesAppliedCount := 0
		for _, change := range mdiffData.Changes {
			if change.FilePath == "" {
				fmt.Fprintln(os.Stderr, "Warning: Skipping a change entry due to missing 'file_path'.")
				continue
			}
			// Content can be empty, meaning the file should be emptied or created empty.

			// filePath from JSON is used as-is. If relative, it's relative to CWD.
			err = writeInPlace(change.FilePath, []byte(change.Content))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing file '%s': %v\n", change.FilePath, err)
				os.Exit(1) // Or collect errors and report at the end
			}
			fmt.Fprintf(os.Stdout, "Successfully applied changes to %s\n", change.FilePath)
			filesAppliedCount++
		}

		if filesAppliedCount == 0 {
			// This case might be hit if all changes had empty file_paths,
			// or if mdiffData.Changes was initially empty (already handled).
			fmt.Fprintln(os.Stderr, "Warning: No file changes were actually applied from the JSON file.")
		} else {
			fmt.Fprintf(os.Stdout, "Successfully applied %d file(s).\n", filesAppliedCount)
		}

	case "extract":
		extractCmd := flag.NewFlagSet("extract", flag.ExitOnError)
		gitignorePathFlag := extractCmd.String("gitignore", "", "Path to a custom .gitignore file. If not provided,\n.gitignore in <directory_path> is used if it exists.")

		extractCmd.Usage = func() { printExtractUsage(extractCmd) }

		err := extractCmd.Parse(os.Args[2:])
		if err != nil {
			os.Exit(1)
		}

		if extractCmd.NArg() < 2 {
			fmt.Fprintln(os.Stderr, "Error: Missing <directory_path> or <file_extensions> for extract command.")
			extractCmd.Usage()
			os.Exit(1)
		}

		directoryPath := extractCmd.Arg(0)
		extensionsStr := extractCmd.Arg(1)
		rawExtensions := strings.Split(extensionsStr, ",")
		var extensions []string
		for _, ext := range rawExtensions {
			trimmedExt := strings.TrimSpace(ext)
			if trimmedExt != "" {
				// Ensure extensions start with a dot if not already
				if !strings.HasPrefix(trimmedExt, ".") {
					trimmedExt = "." + trimmedExt
				}
				extensions = append(extensions, trimmedExt)
			}
		}
		if len(extensions) == 0 {
			fmt.Fprintln(os.Stderr, "Error: No valid file extensions provided.")
			extractCmd.Usage()
			os.Exit(1)
		}

		absScanDir, err := filepath.Abs(directoryPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting absolute path for directory '%s': %v\n", directoryPath, err)
			os.Exit(1)
		}

		dirInfo, err := os.Stat(absScanDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Error: Directory '%s' does not exist.\n", absScanDir)
			} else {
				fmt.Fprintf(os.Stderr, "Error accessing directory '%s': %v\n", absScanDir, err)
			}
			os.Exit(1)
		}
		if !dirInfo.IsDir() {
			fmt.Fprintf(os.Stderr, "Error: Path '%s' is not a directory.\n", absScanDir)
			os.Exit(1)
		}

		ignoreMatcher, err := NewIgnoreMatcher(*gitignorePathFlag, absScanDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing gitignore matcher: %v\n", err)
			os.Exit(1)
		}

		extractedContent, err := extractFileContent(absScanDir, extensions, ignoreMatcher)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error extracting content: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(extractedContent)

	default:
		fmt.Fprintf(os.Stderr, "Error: Unknown command \"%s\"\n\n", command)
		printMainUsage()
		os.Exit(1)
	}
}
