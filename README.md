# mac-cache-cleaner

Go CLI for macOS that reclaims disk space by running **safe, official cleanup commands** for common developer tools—never `rm -rf`.

## Features
- Dry-run by default
- Config-driven targets and commands
- Docker, Yarn, npm, pnpm, Go, Maven, Brew, Chrome
- JSON output for automation
- `--init` to scaffold a starter config

## Quick start
```bash
make build
./build/mac-cache-cleaner --init
./build/mac-cache-cleaner --config ~/.config/mac-cache-cleaner/config.yaml      # dry-run
./build/mac-cache-cleaner --config ~/.config/mac-cache-cleaner/config.yaml --clean
./build/mac-cache-cleaner --config ~/.config/mac-cache-cleaner/config.yaml --targets brew,pnpm --clean
```

## Notes
- Chrome is informational only (no stable CLI for cache clear).
- Enable Docker prunes via config `dockerPruneByDefault: true` or `--docker-prune`.
