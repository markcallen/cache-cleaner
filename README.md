# mac-cache-cleaner

Go CLI for macOS that reclaims disk space by running **safe, official cleanup commands** for common developer tools—never `rm -rf`.

## Features
- Dry-run by default
- Config-driven targets and commands
- **Tool requirement checking** - automatically checks if required tools are installed
- **Installation guidance** - provides installation commands when tools are missing
- Docker, Yarn, npm, pnpm, Go, Maven, Brew, Chrome
- JSON output for automation
- `--init` to scaffold a starter config
- Flexible output: summary mode (default) or detailed per-directory breakdown (`--details`)

## Output Modes

By default, the tool shows a summary with total cache size per target:
```bash
./build/mac-cache-cleaner  # Shows: [target] total_size
```

Use `--details` to see per-directory breakdowns:
```bash
./build/mac-cache-cleaner --details  # Shows: [target] total_size
                                       #   - directory1 — size, items, date
                                       #   - directory2 — size, items, date
```

## Quick start
```bash
make build
./build/mac-cache-cleaner --init
./build/mac-cache-cleaner --config ~/.config/mac-cache-cleaner/config.yaml      # dry-run (summary only)
./build/mac-cache-cleaner --config ~/.config/mac-cache-cleaner/config.yaml --details  # show per-directory details
./build/mac-cache-cleaner --config ~/.config/mac-cache-cleaner/config.yaml --clean
./build/mac-cache-cleaner --config ~/.config/mac-cache-cleaner/config.yaml --targets brew,pnpm --clean
./build/mac-cache-cleaner --config ~/.config/mac-cache-cleaner/config.yaml --check-tools  # check if tools are installed
```

## Tool Requirements

The config allows you to specify required tools for each target. Before running commands, the app checks if these tools are installed and in your PATH.

Example:
```yaml
targets:
  - name: docker
    enabled: true
    cmds:
      - - docker
        - system
        - prune
    tools:
      - name: docker
        version: "24.0"      # optional: minimum version
        installCmd: "brew install --cask docker"
        installNotes: "Installation notes (optional)"
```

When a tool is missing, the app will:
- Show a warning with installation instructions
- Mark related commands as "not found"
- Provide the install command (defaults to brew if available)

### Check Tool Status

Use the `--check-tools` flag to quickly verify if all required tools are installed:

```bash
# Check all tools for all enabled targets
./build/mac-cache-cleaner --check-tools

# Check tools for specific targets only
./build/mac-cache-cleaner --check-tools --targets docker,go
```

This will:
- Show ✓ for installed tools
- Show ✗ for missing tools with installation instructions
- List which targets require each tool
- Exit with status 0 if all tools are present, 1 if any are missing

## Notes
- Chrome is informational only (no stable CLI for cache clear).
- Enable Docker prunes via config `dockerPruneByDefault: true` or `--docker-prune`.
