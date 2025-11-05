# Product Requirements Document (PRD): cache-cleaner

## Overview
The cache-cleaner project consists of three cross-platform Go CLI tools designed to help developers reclaim disk space by identifying and cleaning various types of cache directories and developer cruft. Each tool serves a specific purpose and can be used independently or together.

## Project Goals
- Safety first (no direct deletions where possible, use official tool commands)
- Transparent dry-run by default
- Cross-platform support (macOS, Linux, Windows WSL)
- Extensible via YAML configuration
- Transparent size reporting
- Selective filtering for targeted operations
- JSON output for automation/CI integration

---

## mac-cache-cleaner

### Overview
A safe, config-driven Go CLI for macOS that frees disk space using **official tool commands** (Docker, npm/yarn/pnpm, Go, Maven, Brew, and many more) instead of deleting files directly.

### Goals
- Safety first (no direct deletions)
- Transparent dry-run by default
- Extensible via YAML
- Transparent size reporting
- Target filtering for selective operations
- Intelligent tool requirement checking
- Installation guidance for missing tools

### Key Features

#### CLI Flags
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

#### Configuration Structure
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

#### Supported Targets
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
- **terraform** - Terraform plugin cache
- **packer** - Packer plugins directory
- **ollama** - Ollama models and cache (uses official prune)
- **home-cache** - Top-level ~/.cache subdirectories (informational only)
- **pyenv** - Pyenv installed versions and downloads (informational)
- **rustup** - Rustup toolchains and targets (informational)
- **vscode-extensions** - VS Code extensions and data under ~/.vscode (informational)
- **rvm** - RVM installed rubies and archives (informational/has cleanup)
- **dropbox** - Dropbox metadata and state (informational only)
- **cursor** - Cursor editor state and cache (informational)

#### Path Expansion
- Supports `~` for home directory
- Expands environment variables (`$VAR`)
- Supports `$(brew --cache)` expansion
- Glob patterns for flexible matching
- Command substitutions are whitelist-only for safety; currently only `brew --cache` is supported. Other substitutions (e.g., `$(docker ...)`) are rejected.

#### Reporting Features
- Size measurement in bytes with human-readable output
- Item count tracking
- Latest modification time per path
- Command availability detection
- JSON output for programmatic processing
- Warnings for glob errors
- Before/after comparison when `--clean` is used
- Total space freed calculation
- Docker usage is measured via `docker system df` (JSON/template parsing), not by reading `Docker.raw` directly

### Behavior
1. **Dry-run by default** - Reports sizes without making changes
2. **Tool requirement checking** - Checks if required tools are installed before running commands
3. **Size scanning** - Measures all configured paths before any cleanup
4. **Command execution** - Runs official tool CLI commands when `--clean` is set
5. **Target filtering** - Process specific targets with `--targets` flag
6. **Docker prune injection** - Adds prune commands when configured/flagged
7. **Auto-confirm prompts** - Automatically sends 'y' to interactive prompts when running commands
8. **Re-scan after cleanup** - Shows before/after sizes and freed space when `--clean` is used

#### Tool Management
- Before executing commands, the app checks if required tools (specified in `tools` array) are installed
- If a tool is missing, a warning is shown with installation guidance
- Installation commands default to `brew install` if Homebrew is available
- Version checking: When a version is specified, the app performs a basic `--version` contains check to verify presence; it does not perform full semantic version comparison
- Commands are marked as "not found" when their prerequisite tools are missing
- Custom path checking: If `checkPath` is specified, the app checks for the existence of that file instead of using PATH lookup
- Tool status is displayed with ✓ for installed tools and ✗ for missing tools

#### Check Tools Flag
The `--check-tools` flag provides a quick way to verify tool requirements:
- Shows ✓ for installed tools with version info (if available)
- Shows ✗ for missing tools with installation commands
- Lists which targets require each tool
- Respects `--targets` flag to check tools for specific targets only
- Exits with status 0 (all OK) or 1 (some missing) - useful for CI/CD scripts

#### List Targets Flag
The `--list-targets` flag shows all available targets:
- Lists all targets from the config file
- Shows [ENABLED] or [DISABLED] status for each target
- Displays notes for each target
- Useful for discovering available cleanup options

#### JSON Mode Behavior
- When `--json` is provided, the app outputs the initial scan results (including totals and warnings) and exits. Cleanup is not performed even if `--clean` is also set.

### Out of Scope
- Windows/Linux paths (future)
- Direct file deletion (unsafe)
- Chrome deletion automation (no stable CLI)
- Interactive prompts (auto-confirmed with 'y')

### Future Enhancements
- TUI interface
- Homebrew formula
- Scheduled runs
- CI/CD integration examples
- Windows/Linux support

---

## dev-cache

### Overview
A cross-platform Go CLI tool that scans source code directories to find and optionally delete local project cache directories (like `node_modules`, `build`, `dist`, `.venv`, etc.) across multiple programming languages.

### Goals
- Cross-platform support (macOS, Linux, Windows WSL)
- Multi-language cache detection
- Configurable scanning depth and patterns
- Safe deletion with confirmation prompts
- Detailed reporting by language and project

### Key Features

#### CLI Flags
- `--init` creates starter config file and exits
- `--force` forces overwrite of existing config (use with --init)
- `--config PATH` specifies path to YAML config (default: `~/.config/dev-cache/config.yaml`)
- `--scan PATH` directory to scan (overrides config default)
- `--depth N` max scan depth (overrides config default, 0 = use config)
- `--languages LIST` comma-separated list of languages to scan (e.g., `node,python,go`)
- `--clean` deletes found cache directories
- `--yes` skips confirmation prompt for cleanup
- `--json` outputs results as JSON
- `--details` shows detailed per-project breakdown

#### Configuration Structure
```yaml
version: 1
options:
  defaultScanPath: ~/src
  maxDepth: 1  # How many levels deep to scan

languages:
  - name: node
    enabled: true
    patterns:
      - node_modules
      - .npm
      - .yarn
      - .pnpm-store

  - name: python
    enabled: true
    patterns:
      - .venv
      - venv
      - __pycache__
      - .pytest_cache
      - .mypy_cache
```

#### Supported Languages
- **Node.js**: `node_modules`, `.npm`, `.yarn`, `.pnpm-store`
- **Python**: `.venv`, `venv`, `__pycache__`, `.pytest_cache`, `.mypy_cache`, `.tox`
- **Go**: `vendor`
- **Rust**: `target`
- **Java/Kotlin**: `target`, `.gradle`, `build`
- **Next.js**: `.next`, `dist`, `build`, `out`, `.cache`
- **Vue/Nuxt**: `.nuxt`, `dist`, `build`, `out`, `.cache`, `.parcel-cache`
- **PHP**: `vendor`
- **Ruby**: `vendor/bundle`
- **C#/.NET**: `bin`, `obj`
- **C/C++**: `build`, `cmake-build-*` (wildcard supported)
- **Flutter/Dart**: `.dart_tool`

#### Pattern Matching
- **Exact match**: `node_modules` matches only `node_modules`
- **Wildcard prefix**: `cmake-build-*` matches `cmake-build-debug`, `cmake-build-release`, etc.

#### Reporting Features
- Summary mode (default): Groups findings by language with totals
- Detailed mode (`--details`): Shows each cache directory found with project path, cache type, language, size, and item count
- JSON output for programmatic processing
- Size measurement in bytes with human-readable output
- Item count tracking per directory

### Behavior
1. **Dry-run by default** - Reports cache directories without deleting files
2. **Recursive scanning** - Walks through directory tree looking for cache patterns
3. **Language filtering** - Can target specific languages via `--languages` flag
4. **Depth control** - Configurable maximum scan depth to limit traversal
5. **Safe deletion** - Requires explicit `--clean` flag with confirmation prompt (unless `--yes` is used)
6. **Error handling** - Deletion errors are logged and reported, but don't stop the process

### Platform Support
- **macOS**: Native support (amd64 and arm64)
- **Linux**: Full support (amd64 and arm64)
- **Windows WSL**: Full support through Linux compatibility

All paths use `filepath.Join()` for cross-platform compatibility. Home directory expansion (`~`) works on all platforms via `os.UserHomeDir()`.

### Safety Considerations
- **Dry-run by default**: The tool never deletes files unless `--clean` is explicitly provided
- **Confirmation prompt**: When using `--clean`, you must confirm the deletion (unless `--yes` is used)
- **Shows what will be deleted**: The tool displays all findings before asking for confirmation
- **Error handling**: Deletion errors are logged and reported, but don't stop the process

### Future Enhancements
- Integration with version control (skip if in .gitignore)
- Whitelist/blacklist per project
- Age-based filtering (delete only old caches)
- Backup before deletion
- TUI interface

---

## git-cleaner

### Overview
A cross-platform Go CLI tool that scans directories for `.git` directories, reports their sizes, and optionally optimizes repositories using `git gc`.

### Goals
- Cross-platform support (macOS, Linux, Windows WSL)
- Efficient repository discovery
- Safe optimization using official Git commands
- Clear reporting of disk savings

### Key Features

#### CLI Flags
- `--scan PATH` directory to scan for .git directories (required)
- `--clean` runs `git gc` in each repository and shows disk savings

#### Reporting Features
- Table output showing repository paths and `.git` directory sizes
- Item count tracking per repository
- Before/after comparison when `--clean` is used
- Total disk savings calculation with percentage

### Behavior
1. **Recursive scanning** - Walks through directory tree looking for `.git` directories
2. **Size calculation** - Calculates total size by walking through all files in each `.git` directory
3. **Optimization** (with `--clean`) - Runs `git gc` in each repository's parent directory
4. **Rescan after optimization** - After optimization, rescans to calculate disk space saved

### Platform Support
- **macOS**: Native support (amd64 and arm64)
- **Linux**: Full support (amd64 and arm64)
- **Windows WSL**: Full support through Linux compatibility

All paths use `filepath.Join()` for cross-platform compatibility. Home directory expansion (`~`) works on all platforms via `os.UserHomeDir()`.

### Requirements
- Go 1.21 or later
- Git must be installed and available in PATH (for `--clean` functionality)

### Safety Considerations
- Uses official `git gc` command for optimization (safe)
- Only optimizes Git repositories, never deletes them
- Shows clear before/after comparison

### Future Enhancements
- Configuration file support for default scan paths
- Filtering by repository size (only optimize large repos)
- Filtering by last activity date
- Integration with git worktree detection
- Support for Git LFS optimization
- JSON output mode

---

## Common Features Across All Apps

### Cross-Platform Support
All three apps support:
- macOS (amd64 and arm64)
- Linux (amd64 and arm64)
- Windows WSL (via Linux compatibility)

### Development Standards
- Go 1.21+ required
- Consistent Makefile structure
- Pre-commit hooks support
- Test coverage tracking
- Linting with golangci-lint
- Formatting with gofmt

### Output Modes
- Human-readable table output (default)
- JSON output for automation (`--json` flag)
- Detailed mode for granular information (`--details` flag where applicable)

### Safety Features
- Dry-run by default
- Explicit confirmation required for destructive operations
- Clear reporting of what will be affected
- Error handling that doesn't silently fail
