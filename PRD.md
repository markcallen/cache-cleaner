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
2. **Size scanning** - Measures all configured paths before any cleanup
3. **Command execution** - Runs official tool CLI commands when `--clean` is set
4. **Target filtering** - Process specific targets with `--targets` flag
5. **Docker prune injection** - Adds prune commands when configured/flagged

## Out of scope
- Windows/Linux paths (future)
- Direct file deletion (unsafe)
- Chrome deletion automation (no stable CLI)

## Future Enhancements
- TUI interface
- Homebrew formula
- Scheduled runs
- CI/CD integration examples
