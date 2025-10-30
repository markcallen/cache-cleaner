APP_NAME=mac-cache-cleaner
VERSION=1.0.0
BUILD_DIR=build
ZIP_NAME=$(APP_NAME)-$(VERSION).zip
GOLANGCI_LINT_VERSION?=v1.61.0
GOBIN?=$(shell go env GOPATH)/bin

all: fmt lint vet build

build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go mod tidy
	@go build -o $(BUILD_DIR)/$(APP_NAME)

fmt:
	@echo "Formatting Go files..."
	@gofmt -s -w .
	@go fmt ./...

vet:
	@echo "Running go vet..."
	@go vet ./...

lint:
	@echo "Running lint..."
	@command -v golangci-lint >/dev/null 2>&1 \
		&& golangci-lint run \
		|| (echo "golangci-lint not found. Install with 'make install'. Falling back to 'go vet'" && go vet ./...)

install:
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION) to $(GOBIN)..."
	@mkdir -p $(GOBIN)
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN) $(GOLANGCI_LINT_VERSION)

run:
	@$(BUILD_DIR)/$(APP_NAME)

clean:
	@rm -rf $(BUILD_DIR)

package: build
	@zip -r $(ZIP_NAME) $(BUILD_DIR) README.md PRD.md Makefile go.mod main.go

.PHONY: all build run clean package fmt vet lint install
.PHONY: pre-commit-enable pre-commit

pre-commit-enable:
	@echo "Setting up pre-commit hooks..."
	@command -v pre-commit >/dev/null 2>&1 \
		|| (echo "pre-commit not found. Attempting to install via Homebrew..." && brew install pre-commit)
	@pre-commit install

pre-commit:
	@pre-commit run --all-files
