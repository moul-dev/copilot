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

# --- Test Runner Globals ---
ALL_TESTS_PASSED=true

# --- Apply Test Runner ---
run_apply_test_case() {
    local test_name=$1
    local test_case_dir="$TEST_DIR/$test_name"
    echo "--- Running apply test: $test_name ---"

    local original_file="$test_case_dir/original.txt"
    local changes_json="$test_case_dir/changes.json"
    local expected_file="$test_case_dir/expected.txt"
    local output_file="$test_case_dir/test_output.txt"

    # Prepare for the test by copying the original file
    cp "$original_file" "$output_file"

    # Run the apply command
    if ! "$BINARY_PATH" apply "$changes_json" > /dev/null; then
        echo -e "${RED}FAIL: $test_name (Command exited with an error)${NC}"
        ALL_TESTS_PASSED=false
        rm "$output_file"
        return
    fi

    # Compare the actual output with the expected result
    if diff --strip-trailing-cr "$expected_file" "$output_file" > /dev/null; then
        echo -e "${GREEN}PASS: Output matches expected result.${NC}"
    else
        echo -e "${RED}FAIL: Output does not match expected result.${NC}"
        echo "Differences:"
        diff --strip-trailing-cr "$expected_file" "$output_file"
        ALL_TESTS_PASSED=false
    fi

    # Clean up
    rm "$output_file"
    echo ""
}

# --- Extract Test Runner ---
run_extract_test_case() {
    local test_name=$1
    local test_case_dir="$TEST_DIR/$test_name"
    echo "--- Running extract test: $test_name ---"

    local args_file="$test_case_dir/extract_args.txt"
    local expected_file="$test_case_dir/expected.txt"
    local output_file="$test_case_dir/test_output.txt"

    # Read arguments from file into an array, one arg per line
    mapfile -t args < "$args_file"

    # Run the extract command
    if ! "$BINARY_PATH" extract "${args[@]}" > "$output_file"; then
        echo -e "${RED}FAIL: $test_name (Command exited with an error)${NC}"
        ALL_TESTS_PASSED=false
        rm "$output_file"
        return
    fi

    # Compare the actual output with the expected result
    if diff --strip-trailing-cr "$expected_file" "$output_file" > /dev/null; then
        echo -e "${GREEN}PASS: Output matches expected result.${NC}"
    else
        echo -e "${RED}FAIL: Output does not match expected result.${NC}"
        echo "Differences:"
        diff --strip-trailing-cr "$expected_file" "$output_file"
        ALL_TESTS_PASSED=false
    fi

    # Clean up
    rm "$output_file"
    echo ""
}


# --- Discover and Run All Tests ---
echo "Discovering and running tests..."
for test_case_dir in "$TEST_DIR"/*/; do
    test_name=$(basename "$test_case_dir")

    # Decide which runner to use based on file existence
    if [ -f "${test_case_dir}changes.json" ]; then
        run_apply_test_case "$test_name"
    elif [ -f "${test_case_dir}extract_args.txt" ]; then
        run_extract_test_case "$test_name"
    else
        echo "Skipping '$test_name' - no recognized test file found (changes.json or extract_args.txt)."
    fi
done

# --- Final Result ---
if [ "$ALL_TESTS_PASSED" = true ]; then
  echo -e "${GREEN}All tests passed successfully!${NC}"
  exit 0
else
  echo -e "${RED}One or more tests failed.${NC}"
  exit 1
fi