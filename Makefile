APP_NAME=mac-cache-cleaner
VERSION=1.0.0
BUILD_DIR=build
ZIP_NAME=$(APP_NAME)-$(VERSION).zip

all: build

build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go mod tidy
	@go build -o $(BUILD_DIR)/$(APP_NAME)

run:
	@$(BUILD_DIR)/$(APP_NAME)

clean:
	@rm -rf $(BUILD_DIR)

package: build
	@zip -r $(ZIP_NAME) $(BUILD_DIR) README.md PRD.md Makefile go.mod main.go

.PHONY: all build run clean package
