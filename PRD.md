# Product Requirements Document (PRD): mac-cache-cleaner

## Overview
A safe, config-driven Go CLI for macOS that frees disk space using **official tool commands** (Docker, npm/yarn/pnpm, Go, Maven, Brew, and many more) instead of deleting files directly.

## Goals
- Safety first (no direct deletions)
- Transparent dry-run by default
- Extensible via YAML
- Transparent size reporting
- Target filtering for selective operations
- Intelligent tool requirement checking
- Installation guidance for missing tools

## Key Features

### CLI Flags
- `--init` writes starter config to `~/.config/mac-cache-cleaner/config.yaml`
- `--force` forces overwrite of existing config when used with `--init`
- `--config` specifies path to YAML config (defaults to `~/.config/mac-cache-cleaner/config.yaml`)
- `--clean` executes cleanup commands (default: dry-run mode)
- `--targets` filters targets to scan/clean (comma-separated or "all")
- `--details` shows detailed per-directory information
- `--json` outputs structured JSON for automation/CI
- `--list-targets` lists all available targets in the config
- `--docker-prune` injects Docker prune commands at runtime
- `--check-tools` checks if required tools are installed and exits (can be combined with `--targets`)

### Configuration Structure
```yaml
version: 1
options:
  dockerPruneByDefault: false
targets:
  - name: docker
    enabled: true
    notes: "Report Docker.raw size; optionally run prunes."
    paths: ["~/Library/Caches/docker/*", ...]  # measured for size
    cmds: [["docker", "builder", "prune", "-af"], ...]  # executed when --clean
    tools:  # required tools for this target
      - name: docker
        version: "24.0"  # optional: minimum version
        installCmd: "brew install --cask docker"  # installation command
        installNotes: "Optional installation notes"
        checkPath: "~/.docker/config"  # optional: check specific file path instead of PATH
```

### Supported Targets
- **docker** - Docker caches and images
- **brew** - Homebrew packages and caches
- **npm** - npm cache
- **yarn** - Global Yarn cache
- **pnpm** - pnpm store and cache
- **node-versions** - Node version managers (nvm, volta)
- **expo** - Expo and React Native caches
- **go** - Go build & module caches
- **rust** - Rust registry and build caches
- **python** - pip, pipenv, and poetry caches
- **conda** - Conda package and cache cleanup
- **maven** - Maven local repository
- **gradle** - Gradle build caches and wrappers
- **xcode** - Xcode build artifacts and caches
- **ruby** - Ruby and Bundler caches
- **php** - Composer PHP cache
- **dotnet** - .NET SDK and NuGet caches
- **vscode** - VS Code caches and logs
- **jetbrains** - JetBrains IDE caches (IntelliJ, PyCharm, WebStorm, etc.)
- **build-tools** - Compiler and build caches (ccache, bazel, Xcode)
- **chrome** - Chrome caches (informational only, no CLI)
- **macos** - macOS system caches (advanced users only, disabled by default)
- **flutter** - Flutter and Dart caches
- **android** - Android SDK and emulator caches
- **android-studio** - Android Studio IDE caches

### Path Expansion
- Supports `~` for home directory
- Expands environment variables (`$VAR`)
- Supports `$(brew --cache)` expansion
- Glob patterns for flexible matching

### Reporting Features
- Size measurement in bytes with human-readable output
- Item count tracking
- Latest modification time per path
- Command availability detection
- JSON output for programmatic processing
- Warnings for glob errors
- Before/after comparison when `--clean` is used
- Total space freed calculation

## Behavior
1. **Dry-run by default** - Reports sizes without making changes
2. **Tool requirement checking** - Checks if required tools are installed before running commands
3. **Size scanning** - Measures all configured paths before any cleanup
4. **Command execution** - Runs official tool CLI commands when `--clean` is set
5. **Target filtering** - Process specific targets with `--targets` flag
6. **Docker prune injection** - Adds prune commands when configured/flagged
7. **Auto-confirm prompts** - Automatically sends 'y' to interactive prompts when running commands
8. **Re-scan after cleanup** - Shows before/after sizes and freed space when `--clean` is used

### Tool Management
- Before executing commands, the app checks if required tools (specified in `tools` array) are installed
- If a tool is missing, a warning is shown with installation guidance
- Installation commands default to `brew install` if Homebrew is available
- Version checking: If a version is specified, the app verifies the tool meets the minimum version requirement
- Commands are marked as "not found" when their prerequisite tools are missing
- Custom path checking: If `checkPath` is specified, the app checks for the existence of that file instead of using PATH lookup
- Tool status is displayed with ✓ for installed tools and ✗ for missing tools

### Check Tools Flag
The `--check-tools` flag provides a quick way to verify tool requirements:
- Shows ✓ for installed tools with version info (if available)
- Shows ✗ for missing tools with installation commands
- Lists which targets require each tool
- Respects `--targets` flag to check tools for specific targets only
- Exits with status 0 (all OK) or 1 (some missing) - useful for CI/CD scripts

### List Targets Flag
The `--list-targets` flag shows all available targets:
- Lists all targets from the config file
- Shows [ENABLED] or [DISABLED] status for each target
- Displays notes for each target
- Useful for discovering available cleanup options

## Out of scope
- Windows/Linux paths (future)
- Direct file deletion (unsafe)
- Chrome deletion automation (no stable CLI)
- Interactive prompts (auto-confirmed with 'y')

## Future Enhancements
- TUI interface
- Homebrew formula
- Scheduled runs
- CI/CD integration examples
- Windows/Linux support
