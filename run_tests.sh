#!/bin/bash

# Exit immediately if a command exits with a non-zero status.
set -e

# --- Configuration ---
BINARY_PATH="./bin/copilot"
TEST_DIR="tests"

# --- Colors for output ---
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# --- Build the binary ---
echo "Building the 'copilot' binary..."
if ! make build > /dev/null; then
    echo -e "${RED}Build failed. Exiting.${NC}"
    exit 1
fi
echo "Build complete."
echo ""

# --- Test Runner Function ---
ALL_TESTS_PASSED=true

run_test_case() {
    local test_name=$1
    local test_case_dir="$TEST_DIR/$test_name"

    local original_file="$test_case_dir/original.txt"
    local changes_json="$test_case_dir/changes.json"
    local expected_file="$test_case_dir/expected.txt"
    local output_file="$test_case_dir/test_output.txt"

    echo "--- Running test: $test_name ---"

    # 1. Prepare for the test by copying the original file to the output path.
    #    The 'file_path' inside changes.json points to this output file.
    cp "$original_file" "$output_file"

    # 2. Run the apply command.
    if ! "$BINARY_PATH" apply "$changes_json" > /dev/null; then
        echo -e "${RED}FAIL: $test_name (Command exited with an error)${NC}"
        ALL_TESTS_PASSED=false
        rm "$output_file" # Clean up
        return
    fi

    # 3. Compare the actual output with the expected result.
    #    Using `diff --strip-trailing-cr` to handle potential line ending differences.
    if diff --strip-trailing-cr "$expected_file" "$output_file" > /dev/null; then
        echo -e "${GREEN}PASS: Output matches expected result.${NC}"
    else
        echo -e "${RED}FAIL: Output does not match expected result.${NC}"
        echo "Differences:"
        diff --strip-trailing-cr "$expected_file" "$output_file"
        ALL_TESTS_PASSED=false
    fi

    # 4. Clean up the generated output file.
    rm "$output_file"
    echo ""
}

# --- Discover and Run All Tests ---
# Loop through each subdirectory in the tests directory
for test_case in "$TEST_DIR"/*/; do
    # Remove trailing slash to get the directory name
    test_case_name=$(basename "$test_case")
    run_test_case "$test_case_name"
done

# --- Final Result ---
if [ "$ALL_TESTS_PASSED" = true ]; then
  echo -e "${GREEN}All tests passed successfully!${NC}"
  exit 0
else
  echo -e "${RED}One or more tests failed.${NC}"
  exit 1
fi