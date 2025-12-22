# cache-cleaner

A collection of Go CLI tools for reclaiming disk space on development machines.

## Apps

### mac-cache-cleaner
macOS cache cleaner that runs **safe, official cleanup commands** for developer tools (Docker, npm, Homebrew, etc.)—never `rm -rf`.

### dev-cache
Cross-platform tool that scans source directories for project cache directories (like `node_modules`, `.venv`, `target`, etc.) across multiple languages.

### git-cleaner
Cross-platform tool that finds `.git` directories, reports their sizes, and optionally optimizes repositories with `git gc`.

## Install

### Recommended: Homebrew (macOS)

```bash
brew tap markcallen/cache-cleaner
brew install cache-cleaner
```

This installs all 3 apps: `dev-cache`, `git-cleaner`, and `mac-cache-cleaner`.

### Alternative: Direct Download

If you don't have Homebrew, you can use the install script:

```bash
# Install all 3 apps to ~/.local/bin
curl -sSfL https://raw.githubusercontent.com/markcallen/cache-cleaner/HEAD/install.sh | sh -s -- -b $HOME/.local/bin

# Install specific app only
curl -sSfL https://raw.githubusercontent.com/markcallen/cache-cleaner/HEAD/install.sh | sh -s -- -b $HOME/.local/bin -a mac-cache-cleaner
```

**Note**: Make sure `~/.local/bin` is in your PATH:
```bash
export PATH="$HOME/.local/bin:$PATH"
```

Add this to your shell profile (`~/.zshrc` or `~/.bashrc`) to make it permanent.

## Quick Start

### mac-cache-cleaner
```bash
mac-cache-cleaner --init
mac-cache-cleaner                    # dry-run (summary)
mac-cache-cleaner --details          # show per-directory details
mac-cache-cleaner --clean            # execute cleanup
mac-cache-cleaner --targets brew,npm --clean
```

### dev-cache
```bash
dev-cache --init
dev-cache                            # scan for cache directories (dry-run)
dev-cache --details                  # detailed breakdown
dev-cache --scan ~/projects --clean  # delete found caches
```

### git-cleaner
```bash
git-cleaner --scan ~/src             # find and report .git sizes
git-cleaner --scan ~/src --clean     # optimize with git gc
```

## Documentation

Each app has detailed documentation in its own directory:
- [`mac-cache-cleaner/README.md`](mac-cache-cleaner/README.md) - Full mac-cache-cleaner docs
- [`dev-cache/README.md`](dev-cache/README.md) - Full dev-cache docs
- [`git-cleaner/README.md`](git-cleaner/README.md) - Full git-cleaner docs

## Development

### Prerequisites

To develop on this project, you need the following tools installed:

- **Go** 1.21 or higher ([download](https://go.dev/dl/))
- **Make** (typically pre-installed on macOS/Linux)
- **GoReleaser** (optional, for releases): `brew install goreleaser`
- **pre-commit** (optional, for Git hooks): `pip install pre-commit`

You can verify all required tools are installed by running:
```bash
make check-tools
```

### Building

Build all apps:
```bash
make all
```

Build specific app:
```bash
make dev-cache
make git-cleaner
make mac-cache-cleaner
```

Run tests/lint for all:
```bash
make test
make lint
make vet
```

## Questions?

Open an issue on GitHub: https://github.com/markcallen/cache-cleaner/issues

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Author

Mark C Allen ([@markcallen](https://github.com/markcallen))
