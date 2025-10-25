# Product Requirements Document (PRD): mac-cache-cleaner

## Overview
A safe, config-driven Go CLI for macOS that frees disk space using **official tool commands** (Docker, npm/yarn/pnpm, Go, Maven, Brew) instead of deleting files directly.

## Goals
- Safety first (no direct deletions)
- Transparent dry-run by default
- Extensible via YAML
- Transparent size reporting
- Target filtering for selective operations

## Key Features

### CLI Flags
- `--init` writes starter config to `~/.config/mac-cache-cleaner/config.yaml`
- `--config` specifies path to YAML config (defaults to `~/.config/mac-cache-cleaner/config.yaml`)
- `--clean` executes cleanup commands (default: dry-run mode)
- `--targets` filters targets to scan/clean (comma-separated or "all")
- `--json` outputs structured JSON for automation/CI
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
```

### Supported Targets
- **docker** - Docker caches and images
- **yarn** - Global Yarn cache
- **npm** - npm cache
- **pnpm** - pnpm store and cache
- **go** - Go build & module caches
- **maven** - Maven local repository
- **brew** - Homebrew packages and caches
- **chrome** - Chrome caches (informational only, no CLI)

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

## Behavior
1. **Dry-run by default** - Reports sizes without making changes
2. **Tool requirement checking** - Checks if required tools are installed before running commands
3. **Size scanning** - Measures all configured paths before any cleanup
4. **Command execution** - Runs official tool CLI commands when `--clean` is set
5. **Target filtering** - Process specific targets with `--targets` flag
6. **Docker prune injection** - Adds prune commands when configured/flagged

### Tool Management
- Before executing commands, the app checks if required tools (specified in `tools` array) are installed
- If a tool is missing, a warning is shown with installation guidance
- Installation commands default to `brew install` if Homebrew is available
- Version checking: If a version is specified, the app verifies the tool meets the minimum version requirement
- Commands are marked as "not found" when their prerequisite tools are missing

### Check Tools Flag
The `--check-tools` flag provides a quick way to verify tool requirements:
- Shows ✓ for installed tools with version info (if available)
- Shows ✗ for missing tools with installation commands
- Lists which targets require each tool
- Respects `--targets` flag to check tools for specific targets only
- Exits with status 0 (all OK) or 1 (some missing) - useful for CI/CD scripts

## Out of scope
- Windows/Linux paths (future)
- Direct file deletion (unsafe)
- Chrome deletion automation (no stable CLI)

## Future Enhancements
- TUI interface
- Homebrew formula
- Scheduled runs
- CI/CD integration examples
