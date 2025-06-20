# Moul Copilot

The `copilot` CLI tool is designed to streamline the process of applying Large Language Model (LLM) generated code changes to your local file system. It serves as a practical middle ground between fully autonomous AI agents, which can sometimes feel disconnected from the development workflow, and the manual, error-prone process of copying and pasting code snippets from LLM responses.

This tool helps bridge that gap by:

1.  **Extracting context:** Allowing you to easily gather relevant file contents from your project to provide as context to an LLM.
2.  **Applying changes:** Providing a structured way to apply multi-file changes generated by an LLM (in a specific JSON format) directly to your project.

This approach aims to keep you in control while leveraging the power of LLMs for code generation and modification.

## Commands

The `copilot` tool provides the following commands:

### 1. `extract`

Extracts content from specified files within a directory, respecting `.gitignore` rules. This is useful for gathering context to feed into an LLM.

**Usage:**

```bash
copilot extract [options] <directory_path> <file_extensions>
```

**Arguments:**

- `<directory_path>`: Path to the directory to scan (e.g., `./src`).
- `<file_extensions>`: Comma-separated list of file extensions to include (e.g., `.go,.md,.js`). Extensions should include the dot.

**Options:**

- `--gitignore <path>`: Path to a custom `.gitignore` file. If not provided, the tool looks for a `.gitignore` file in `<directory_path>`.

**Output Format:**
The `extract` command outputs the content of the matched files to standard output, with each file's content wrapped in tags:

```
<file_path>path/to/relative/file1.ext</file_path>
Content of file1...
<file_path_end>path/to/relative/file1.ext</file_path_end>

<file_path>path/to/another/file2.ext</file_path>
Content of file2...
<file_path_end>path/to/another/file2.ext</file_path_end>
```

**Example:**
To extract all `.go` and `.mod` files from the `./myproject` directory, using the `.gitignore` file located at `./myproject/.gitignore`, and save the output to `context.txt`:

```bash
copilot extract ./myproject .go,.mod > context.txt
```

To use a custom ignore file:

```bash
copilot extract --gitignore ./.custom_ignore ./myproject .ts,.tsx > context.txt
```

### 2. `apply`

Applies file content changes from a JSON file. This command reads the JSON file, parses the specified file paths and their new content, and writes the content to the target files. It will create parent directories for the files if they don't already exist.

**Usage:**

```bash
copilot apply <json_file>
```

**Arguments:**

- `<json_file>`: Path to the JSON file containing the file changes.

**JSON Format:**
The JSON file must contain a single JSON object with a top-level key named `changes`. The value of `changes` must be an array of objects, where each object represents a file to be modified and has two keys:

- `file_path` (string): The path to the file that should be created or overwritten. Paths are typically relative to the current working directory where `copilot apply` is executed.
- `content` (string): The new, complete content for the file.

**Example JSON content (`changes.json`):**

```json
{
  "changes": [
    {
      "file_path": "src/service/user.go",
      "content": "package service\n\n// New user service logic\nfunc GetUserName(id int) string {\n    return \"User \" + string(id)\n}"
    },
    {
      "file_path": "README.md",
      "content": "# Project Alpha\n\nUpdated README content."
    }
  ]
}
```

**Example `apply` command:**
To apply the changes defined in `changes.json` to your project:

```bash
copilot apply ./changes.json
```

This will overwrite `src/service/user.go` and `README.md` with the content specified in `changes.json`. If the `src/service/` directory does not exist, it will be created.
