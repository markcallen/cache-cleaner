# mac-cache-cleaner

A macOS-specific Go CLI tool that scans and cleans developer tool caches (Docker, npm, Homebrew, etc.) using **safe, official cleanup commands**—never `rm -rf`.

## Features

- **macOS-specific**: Optimized for macOS developer workflows
- **Safe cleanup**: Only runs official CLI commands (e.g., `docker system prune`, `brew cleanup`, `npm cache clean`)
- **Never destructive**: Never uses `rm -rf` or direct file deletion
- **Configurable targets**: Enable/disable specific cache types (Docker, npm, Homebrew, etc.)
- **Dry-run by default**: Reports disk usage without deleting files
- **Tool checking**: Verifies required tools are installed before cleanup
- **Table output**: Clean summary view or detailed per-directory breakdown
- **JSON output**: Machine-readable output for automation

## Installation

### From Source

```bash
git clone <repository>
cd mac-cache-cleaner
make build
```

The binary will be in `build/mac-cache-cleaner`.

### Cross-compilation

Build for multiple platforms:

```bash
make build-all  # Builds for Linux (amd64/arm64) and macOS (amd64/arm64)
```

Or build individually:

```bash
make build-linux   # Linux builds only
make build-darwin  # macOS builds only
```

## Quick Start

1. **Initialize config**:
```bash
./build/mac-cache-cleaner --init
```

This creates a config file at `~/.config/mac-cache-cleaner/config.yaml`.

2. **Scan for cache usage** (dry-run):
```bash
./build/mac-cache-cleaner
```

3. **See detailed breakdown**:
```bash
./build/mac-cache-cleaner --details
```

4. **Clean specific targets**:
```bash
./build/mac-cache-cleaner --targets brew,npm --clean
```

5. **Clean all enabled targets**:
```bash
./build/mac-cache-cleaner --clean
```

## Command-Line Arguments

| Flag | Description |
|------|-------------|
| `--init` | Write a starter config to `--config` and exit |
| `--force` | Force overwrite existing config (use with --init) |
| `--config PATH` | Path to YAML config (default: `~/.config/mac-cache-cleaner/config.yaml`) |
| `--targets LIST` | Comma-separated targets to scan/clean (or 'all') |
| `--clean` | Run safe CLI clean commands (default: dry-run) |
| `--json` | Output results as JSON |
| `--details` | Show detailed per-directory information |
| `--list-targets` | List all available targets and exit |
| `--check-tools` | Check if required tools are installed and exit |
| `--docker-prune` | Add docker prune commands at runtime |

## Configuration

The config file defines which cache types to scan and which commands to run:

```yaml
version: 1
options:
  dockerPruneByDefault: false

targets:
  - name: docker
    enabled: true
    notes: "Docker caches and images (safe CLI prune only)"
    paths:
      - ~/Library/Caches/docker
      - ~/Library/Caches/buildx
    cmds:
      - [docker, builder, prune, -af]
      - [docker, system, prune, -af, --volumes]
    tools:
      - name: docker
        installCmd: brew install --cask docker

  - name: brew
    enabled: true
    notes: "Homebrew cleanup (removes old packages and caches)"
    paths:
      - ~/Library/Caches/Homebrew
      - $(brew --cache)
    cmds:
      - [brew, cleanup, -s]
      - [brew, autoremove]
    tools:
      - name: brew
        installCmd: /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

  - name: npm
    enabled: true
    notes: "npm cache"
    paths:
      - ~/.npm
    cmds:
      - [npm, cache, clean, --force]
    tools:
      - name: npm
        installCmd: brew install node
```

### Default Targets

The starter config includes targets for:

- **Docker**: Docker caches, buildx cache, and Docker.raw file
- **Homebrew**: Homebrew cache and cleanup
- **Node.js**: npm, yarn, pnpm, node-versions (nvm)
- **Python**: pip, pipenv, Poetry, uv, conda, pyenv
- **Go**: Go build and module caches
- **Rust**: Cargo registry and build caches (with cargo-cache)
- **Java**: Maven and Gradle caches
- **Ruby**: gem and Bundler caches
- **PHP**: Composer cache
- **.NET**: NuGet packages cache
- **Xcode**: DerivedData, Archives, ModuleCache
- **IDEs**: VS Code, JetBrains, Android Studio caches
- **Build tools**: ccache, bazel
- **Other**: Flutter, Android SDK, Terraform, Packer, Ollama, etc.

### Path Expansion

Paths support:
- **Home directory**: `~` expands to your home directory
- **Environment variables**: `$HOME`, `$GOPATH`, etc.
- **Command substitution**: `$(brew --cache)` expands to Homebrew's cache directory
- **Glob patterns**: `~/Library/Caches/JetBrains/*` matches multiple directories

### Tool Checking

Each target can specify required tools. Use `--check-tools` to verify all required tools are installed:

```bash
./build/mac-cache-cleaner --check-tools
```

This will show which tools are missing and provide installation commands.

## Examples

### List available targets

```bash
./build/mac-cache-cleaner --list-targets
```

### Check tool requirements

```bash
./build/mac-cache-cleaner --check-tools
```

### Scan specific targets

```bash
./build/mac-cache-cleaner --targets brew,npm,yarn
```

### Clean specific targets

```bash
./build/mac-cache-cleaner --targets docker,brew --clean
```

### Use custom config location

```bash
./build/mac-cache-cleaner --config /path/to/config.yaml
```

### JSON output for scripting

```bash
./build/mac-cache-cleaner --json > cache-report.json
```

### Detailed view

```bash
./build/mac-cache-cleaner --details
```

## Output Modes

### Summary Mode (default)

Shows a table with target, disk usage, and clean commands:

```
+----------+-------------+--------------------------+
| Target   | Used        | Clean Commands           |
+----------+-------------+--------------------------+
| docker   | 12.34 GB   | docker builder prune -af |
| brew     | 1.23 GB    | brew cleanup -s          |
| npm      | 456.78 MB  | npm cache clean --force  |
+----------+-------------+--------------------------+
```

### Detailed Mode (`--details`)

Shows individual directories and files:

```
[docker] 12.34 GB
  ~/Library/Caches/docker: 10.23 GB
  ~/Library/Caches/buildx: 2.11 GB
  Commands:
    - docker builder prune -af [ok]
    - docker system prune -af --volumes [ok]

[brew] 1.23 GB
  ~/Library/Caches/Homebrew: 1.23 GB
  Commands:
    - brew cleanup -s [ok]
    - brew autoremove [ok]
```

### After Cleanup

When using `--clean`, shows before/after comparison:

```
After cleanup:

+----------+-------------+-------------+
| Target   | Used        | Freed       |
+----------+-------------+-------------+
| docker   | 2.34 GB    | 10.00 GB    |
| brew     | 500.00 MB  | 730.00 MB   |
+----------+-------------+-------------+

Total space freed: 10.73 GB
```

## Safety Considerations

- **Safe commands only**: Only runs official CLI cleanup commands (e.g., `docker system prune`, `brew cleanup`)
- **Never destructive**: Never uses `rm -rf` or direct file deletion
- **Dry-run by default**: Reports disk usage without deleting files
- **Tool verification**: Checks if required tools are installed before running commands
- **Interactive confirmation**: Commands that require confirmation automatically send 'y' to stdin

## Platform Support

This tool is designed for **macOS** but may work on Linux in some cases. It uses macOS-specific paths like:
- `~/Library/Caches`
- `~/Library/Developer/Xcode`
- `~/Library/Application Support`

For cross-platform cache cleaning, see [`dev-cache`](../dev-cache/README.md).

## Requirements

- Go 1.21 or later
- macOS (primary platform)
- Required tools vary by target (Docker, npm, Homebrew, etc.)

## Development

### Build

```bash
make build
```

### Test

```bash
make test
```

### Format, Lint, Vet

```bash
make fmt
make lint
make vet
```

Or run all checks:

```bash
make all
```

## License

See [LICENSE](../LICENSE) in the parent directory.
