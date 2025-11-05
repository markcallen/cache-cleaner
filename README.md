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

Install all 3 apps (or specific ones) with the install script:

```bash
# Install all 3 apps (latest)
curl -sSfL https://raw.githubusercontent.com/markcallen/cache-cleaner/HEAD/install.sh | sh -s --

# Install specific app only
curl -sSfL https://raw.githubusercontent.com/markcallen/cache-cleaner/HEAD/install.sh | sh -s -- -a mac-cache-cleaner
```

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

## License

See [LICENSE](./LICENSE).
