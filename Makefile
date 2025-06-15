# Binary name
BINARY_NAME=copilot

.PHONY: all build install clean

# Default target: build the binary locally
all: build

# Build the binary in the current directory (module root)
build:
	@echo "Building $(BINARY_NAME) locally..."
	@go build -o ./bin/$(BINARY_NAME) ./
	@echo "$(BINARY_NAME) built as ./$(BINARY_NAME)."

# Install the binary using 'go install'
# 'go install' will build and place the binary in the correct GOBIN or GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	@go install .
	@echo "$(BINARY_NAME) installed successfully."
	@echo "Make sure '$(shell go env GOPATH)/bin', '$(shell go env GOBIN)', or '$(shell go env HOME)/go/bin' is in your PATH."

# Clean build artifacts (only the locally built binary from 'make build')
clean:
	@echo "Cleaning local build artifacts..."
	@rm -f $(BINARY_NAME)
	@echo "Cleaned."
