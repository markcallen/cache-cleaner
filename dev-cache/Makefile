APP_NAME=dev-cache
VERSION=1.0.0
BUILD_DIR=build
GOLANGCI_LINT_VERSION?=v2.6.0
GOBIN?=$(shell go env GOPATH)/bin

all: fmt lint vet build

test:
	@echo "Running tests..."
	@go test ./...

build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go mod tidy
	@go build -o $(BUILD_DIR)/$(APP_NAME) main.go

build-linux:
	@echo "Building $(APP_NAME) for Linux..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 main.go
	@GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 main.go

build-darwin:
	@echo "Building $(APP_NAME) for macOS..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 main.go
	@GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 main.go

build-all: build-linux build-darwin build

fmt:
	@echo "Formatting Go files..."
	@gofmt -s -w .
	@go fmt ./...

vet:
	@echo "Running go vet..."
	@go vet ./...

lint:
	@echo "Running lint..."
	@if [ -x "$(GOBIN)/golangci-lint" ]; then \
		"$(GOBIN)/golangci-lint" run --tests; \
	else \
		command -v golangci-lint >/dev/null 2>&1 && golangci-lint run --fast || (echo "golangci-lint not found. Install with 'make install'."; exit 1) ; \
	fi

install:
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION) to $(GOBIN)..."
	@mkdir -p $(GOBIN)
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(GOBIN) $(GOLANGCI_LINT_VERSION)

run:
	@$(BUILD_DIR)/$(APP_NAME)

clean:
	@rm -rf $(BUILD_DIR)

.PHONY: all build build-linux build-darwin build-all run clean fmt vet lint install test
