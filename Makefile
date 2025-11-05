# Root Makefile - builds all applications
.PHONY: all dev-cache git-cleaner mac-cache-cleaner clean test fmt lint vet build

# Default target - build all applications
all: dev-cache git-cleaner mac-cache-cleaner

# Build each application
dev-cache:
	@echo "Building dev-cache..."
	@$(MAKE) -C dev-cache all

git-cleaner:
	@echo "Building git-cleaner..."
	@$(MAKE) -C git-cleaner all

mac-cache-cleaner:
	@echo "Building mac-cache-cleaner..."
	@$(MAKE) -C mac-cache-cleaner all

# Convenience targets that run in all directories
test:
	@echo "Running tests in all applications..."
	@$(MAKE) -C dev-cache test
	@$(MAKE) -C git-cleaner test
	@$(MAKE) -C mac-cache-cleaner test

fmt:
	@echo "Formatting all applications..."
	@$(MAKE) -C dev-cache fmt
	@$(MAKE) -C git-cleaner fmt
	@$(MAKE) -C mac-cache-cleaner fmt

lint:
	@echo "Linting all applications..."
	@$(MAKE) -C dev-cache lint
	@$(MAKE) -C git-cleaner lint
	@$(MAKE) -C mac-cache-cleaner lint

vet:
	@echo "Running go vet on all applications..."
	@$(MAKE) -C dev-cache vet
	@$(MAKE) -C git-cleaner vet
	@$(MAKE) -C mac-cache-cleaner vet

build:
	@echo "Building all applications..."
	@$(MAKE) -C dev-cache build
	@$(MAKE) -C git-cleaner build
	@$(MAKE) -C mac-cache-cleaner build

clean:
	@echo "Cleaning all applications..."
	@$(MAKE) -C dev-cache clean
	@$(MAKE) -C git-cleaner clean
	@$(MAKE) -C mac-cache-cleaner clean
