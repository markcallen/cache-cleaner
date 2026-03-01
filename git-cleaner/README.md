# git-cleaner

[![CI](https://github.com/markcallen/cache-cleaner/actions/workflows/ci.yml/badge.svg)](https://github.com/markcallen/cache-cleaner/actions/workflows/ci.yml)
[![Release](https://github.com/markcallen/cache-cleaner/actions/workflows/release.yml/badge.svg)](https://github.com/markcallen/cache-cleaner/actions/workflows/release.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/markcallen/cache-cleaner?filename=dev-cache%2Fgo.mod)](https://go.dev/)
[![License](https://img.shields.io/github/license/markcallen/cache-cleaner)](LICENSE)
[![GitHub Release](https://img.shields.io/github/v/release/markcallen/cache-cleaner)](https://github.com/markcallen/cache-cleaner/releases)

A cross-platform Go CLI tool that scans directories for `.git` directories, reports their sizes, and optionally optimizes repositories using `git gc`.

## Features

- **Cross-platform**: Works on macOS, Linux, and Windows WSL
- **Scan for repositories**: Recursively finds all `.git` directories in a specified path
- **Size reporting**: Shows disk usage for each repository's `.git` directory
- **Optimization**: Runs `git gc` to optimize repositories and shows disk savings
- **Table output**: Clean, formatted table showing repository paths and sizes
- **JSON output**: Machine-readable output for scripts and CI
- **Config support**: Optional YAML config with default scan path (`--init`, `--config`)

## Installation

### From Source

```bash
git clone <repository>
cd git-cleaner
make build
```

The binary will be in `build/git-cleaner`.

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

1. **Scan for repositories** (dry-run):
```bash
./build/git-cleaner --scan ~/src
```

2. **Optimize repositories**:
```bash
./build/git-cleaner --scan ~/src --clean
```

## Command-Line Arguments

| Flag | Description |
|------|-------------|
| `--scan PATH` | Directory to scan for .git directories (overrides config) |
| `--clean` | Run `git gc` in each repository and show disk savings |
| `--yes` | Skip cleanup confirmation prompt |
| `--json` | Output structured JSON (no table/prose on stdout) |
| `--show-pct` | Add `.git` size as % of total repo size |
| `--config PATH` | Config path (default: `~/.config/git-cleaner/config.yaml`) |
| `--init` | Write starter config and exit |
| `--force` | Overwrite existing config when used with `--init` |

## Examples

### Scan a specific directory

```bash
./build/git-cleaner --scan ~/projects
```

### Scan and optimize repositories

```bash
./build/git-cleaner --scan ~/projects --clean
```

### Initialize config and scan with defaults

```bash
./build/git-cleaner --init
./build/git-cleaner
```

### JSON output for automation

```bash
./build/git-cleaner --scan ~/projects --json
./build/git-cleaner --scan ~/projects --clean --yes --json
```

This will:
1. Scan for all `.git` directories
2. Display their sizes in a table
3. Run `git gc` in each repository's parent directory
4. Rescan and show the disk savings achieved

### Output Example

```
Scanning /Users/me/src for .git directories...

Found 5 repositories:

+----------------------------+----------+-------+
| Repository Path            | .git Size| Items |
+----------------------------+----------+-------+
| /Users/me/src/project1     | 45.23 MB | 1234  |
| /Users/me/src/project2     | 12.34 MB | 567   |
| /Users/me/src/project3     | 8.90 MB  | 234   |
+----------------------------+----------+-------+
| TOTAL                     | 66.47 MB | 2035  |
+----------------------------+----------+-------+
```

After running with `--clean`:

```
Disk savings: 12.34 MB (18.57%)
Cleaned 5 repositories
```

## How It Works

1. **Scanning**: Recursively walks through the specified directory tree, looking for directories named `.git`
2. **Size calculation**: For each `.git` directory found, calculates the total size by walking through all files
3. **Optimization** (with `--clean`): Runs `git gc` in each repository's parent directory to optimize the repository
4. **Rescan**: After optimization, rescans to calculate the disk space saved

## Platform Support

This tool works on:
- **macOS**: Native support (amd64 and arm64)
- **Linux**: Full support (amd64 and arm64)
- **Windows WSL**: Full support through Linux compatibility

All paths use `filepath.Join()` for cross-platform compatibility. Home directory expansion (`~`) works on all platforms via `os.UserHomeDir()`.

## Requirements

- Go 1.21 or later
- Git must be installed and available in PATH (for `--clean` functionality)

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
