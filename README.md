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
 - Docker usage measured via `docker system df` (no direct file deletions)
 - Safe path expansion with whitelist-only command substitutions

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

## Command-Line Arguments

| Flag | Description |
|------|-------------|
| `--config` | Path to YAML config file (default: `~/.config/mac-cache-cleaner/config.yaml`) |
| `--clean` | Run safe CLI clean commands (default: dry-run) |
| `--targets` | Comma-separated targets to scan/clean (default: `all`) |
| `--details` | Show detailed per-directory information |
| `--json` | Output results as JSON |
| `--init` | Write a starter config to --config and exit |
| `--force` | Force overwrite existing config (use with --init) |
| `--list-targets` | List all available targets and exit |
| `--check-tools` | Check if required tools are installed and exit |
| `--docker-prune` | Add docker prune commands at runtime |

### JSON Mode Behavior
- When `--json` is provided, the app outputs the initial scan (totals, findings, warnings) as JSON and exits. Cleanup is not performed even if `--clean` is also set.

```bash
# Example: JSON output (initial scan only)
./build/mac-cache-cleaner --config ~/.config/mac-cache-cleaner/config.yaml --json > scan.json
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
./build/mac-cache-cleaner --config ~/.config/mac-cache-cleaner/config.yaml --list-targets  # list all available targets
./build/mac-cache-cleaner --config ~/.config/mac-cache-cleaner/config.yaml --json  # output as JSON
```

## Configuration

### Supported Targets

The starter config includes many common targets:

- docker, brew, npm, yarn, pnpm, node-versions, expo, go, rust, python, conda,
  maven, gradle, xcode, ruby, php, dotnet, vscode, jetbrains, build-tools,
  chrome (informational), macos (advanced, disabled by default), flutter,
  android, android-studio, terraform, packer, ollama, home-cache, pyenv,
  rustup, vscode-extensions, rvm, dropbox, cursor.

Use `--list-targets` to see which are enabled in your current config.

```bash
# List all targets with status and notes
./build/mac-cache-cleaner --config ~/.config/mac-cache-cleaner/config.yaml --list-targets

# Check tools required by selected targets only
./build/mac-cache-cleaner --config ~/.config/mac-cache-cleaner/config.yaml --check-tools --targets docker,go
```

### Config File Structure

```yaml
version: 1  # Config version

options:
  dockerPruneByDefault: false  # Automatically add docker prune commands

targets:
  - name: docker              # Target identifier
    enabled: true             # Enable/disable this target
    notes: "Docker caches and images (safe CLI prune only)"  # Optional description
    
    # Paths to measure for size (supports glob patterns and ~ expansion)
    paths:
      - "~/Library/Caches/docker/*"
      - "~/Library/Caches/buildx/*"
      - "$(brew --cache)/*"  # Dynamic brew cache path (whitelisted substitution)
    
    # Commands to run when --clean is used
    cmds:
      - ["docker", "builder", "prune", "-af"]
      - ["docker", "system", "prune", "-af", "--volumes"]
    
    # Required tools for this target
    tools:
      - name: docker                                    # Tool name to check in PATH
        version: "24.0"                                 # Optional: minimum version requirement
        installCmd: "brew install --cask docker"       # Installation command
        installNotes: "Installation notes (optional)"  # Optional: installation guidance
        checkPath: "~/.docker/config"                   # Optional: specific file path to check instead of PATH
```

### Tool Requirements

The config allows you to specify required tools for each target. Before running commands, the app checks if these tools are installed and in your PATH.

**Tool Configuration Fields:**
- `name`: The tool executable name to check in PATH
- `version`: (Optional) Minimum required version
- `installCmd`: Command to install the tool if missing
- `installNotes`: (Optional) Additional installation guidance
- `checkPath`: (Optional) Specific file path to check instead of using PATH

**Example with multiple tools:**
```yaml
  - name: rust
    enabled: true
    paths:
      - "~/.cargo/registry/*"
      - "~/.cargo/git/*"
    cmds:
      - ["cargo", "cache", "-a"]
    tools:
      - name: cargo
        installCmd: "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"
      - name: cargo-cache
        installCmd: "cargo install cargo-cache"
        installNotes: "Install this after cargo is installed"
```

**Example with custom path check:**
```yaml
  - name: node-versions
    enabled: true
    paths:
      - "~/.nvm/versions/node/*"
    cmds:
      - ["nvm", "cache", "clear"]
    tools:
      - name: nvm
        installCmd: "curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.3/install.sh | bash"
        installNotes: "After installation, restart your terminal"
        checkPath: "~/.nvm/nvm.sh"  # Check for specific file instead of PATH
```

When a tool is missing, the app will:
- Show a warning with installation instructions
- Mark related commands as "not found"
- Provide the install command (defaults to brew if available)
 - In summary output, only show clean commands for tools that are installed; otherwise a note indicates missing tools

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
 - Docker usage is measured via `docker system df` (JSON/template parsing). We do not directly read Docker.raw or delete files.
 - Command substitutions inside paths are whitelist-only. Currently only `$(brew --cache)` is supported for safety; others are rejected.

## License

See [LICENSE](./LICENSE).

## Contact

- Creator: **Mark C Allen** (`@markcallen`)
- Questions or issues: open an issue in this repository.
