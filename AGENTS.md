# AGENTS.md - AI Agent Guidelines for cache-cleaner

## Project Overview

**cache-cleaner** is a Go monorepo with three CLI tools for reclaiming disk space:
- **dev-cache** - Scans for project cache directories (node_modules, .venv, target, etc.)
- **git-cleaner** - Finds .git directories, reports sizes, optimizes with `git gc`
- **mac-cache-cleaner** - Runs safe CLI cleanup commands for developer tools

## Build/Lint/Test Commands

### Root Makefile
| Command | Description |
|---------|-------------|
| `make all` | Build all applications |
| `make test` | Run tests in all apps |
| `make fmt` | Format code in all apps |
| `make lint` | Lint all apps (golangci-lint) |
| `make vet` | Run `go vet` on all apps |
| `make clean` | Clean all build artifacts |

### Per-App Commands (from dev-cache/, git-cleaner/, mac-cache-cleaner/)
| Command | Description |
|---------|-------------|
| `make all` | fmt + lint + vet + build |
| `make test` | `go test ./...` |
| `make build` | Build binary to `./build/` |
| `make fmt` | `gofmt -s -w . && go fmt ./...` |
| `make lint` | Lint with golangci-lint |

### Running a Single Test
```bash
cd dev-cache && go test -v -run TestHuman ./...
cd git-cleaner && go test -v -run TestHumanSize ./...
cd mac-cache-cleaner && go test -v -run TestRunWithRealCommands ./...
```

## Code Style

### Go Version
- Minimum: Go 1.21, CI uses: Go 1.22

### Import Organization
```go
import (
    // Standard library (alphabetized)
    "fmt"
    "os"
    "path/filepath"

    // Third-party packages (alphabetized)
    "github.com/olekukonko/tablewriter"
    yaml "gopkg.in/yaml.v3"
)
```

### Naming Conventions
| Type | Convention | Examples |
|------|------------|----------|
| Variables | camelCase | `flagClean`, `scanPath` |
| Functions | camelCase | `checkVersionFlag`, `inspectPath` |
| Types/Structs | PascalCase | `Config`, `Finding`, `Report` |
| Flag variables | Prefix with `flag` | `flagClean`, `flagJSON` |

### Version Info Pattern
```go
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)
```

### Struct Tags
Use both `yaml` and `json` tags; include `omitempty` for optional fields:
```go
type Finding struct {
    Path      string `json:"path"`
    SizeBytes int64  `json:"size_bytes"`
    Err       string `json:"error,omitempty"`
}
```

### Error Handling
```go
// Wrap errors with context
return fmt.Errorf("loading config: %w", err)

// Early returns
if err != nil {
    return f, err
}

// Collect non-fatal errors and continue
if err != nil {
    errors = append(errors, fmt.Sprintf("%s: %v", path, err))
    continue
}

// Warnings to stderr
fmt.Fprintf(os.Stderr, "warning: %v\n", err)
```

### Testing Patterns
```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name  string
        input int64
        want  string
    }{
        {"zero", 0, "0 B"},
        {"bytes", 100, "100 B"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := function(tt.input)
            if got != tt.want {
                t.Fatalf("got %q, want %q", got, tt.want)
            }
        })
    }
}
```
- Use `t.TempDir()` for test directories (auto-cleanup)
- Use `t.Fatal()` / `t.Fatalf()` for failures
- Save/restore `os.Args` when testing CLI flag parsing

## Project Structure
```
cache-cleaner/
├── dev-cache/             # Dev cache scanner
│   ├── main.go, main_test.go, go.mod, Makefile
├── git-cleaner/           # Git repo optimizer (same structure)
├── mac-cache-cleaner/     # macOS cleanup tool (same structure)
├── .github/workflows/     # CI/CD
├── .goreleaser.yaml       # Release config
└── Makefile               # Root orchestration
```

## Key Design Principles

1. **Dry-run by default** - Report without changes unless `--clean` is specified
2. **Safety first** - Use official tool CLI commands, never direct `rm -rf`
3. **Cross-platform paths** - Use `filepath.Join()`, `os.UserHomeDir()`
4. **Config-driven** - YAML config with `--init` to create starter configs
5. **Consistent CLI** - `--version`, `--help`, `--config`, `--clean`, `--json`
6. **Table output** - Use tablewriter for humans, JSON for automation
7. **Error collection** - Collect errors but continue; report at end

## Common Patterns

### Adding a CLI flag
```go
var flagNewOption bool
func init() {
    flag.BoolVar(&flagNewOption, "new-option", false, "Description")
}
```

### Directory scanning
```go
err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
    if err != nil {
        fmt.Fprintf(os.Stderr, "warning: %s: %v\n", path, err)
        return nil // continue walking
    }
    return nil
})
```

## Code Coverage

- **Minimum**: 60% statement coverage for all apps (dev-cache, git-cleaner, mac-cache-cleaner)
- Run coverage: `go test -cover ./...` from each app directory
- Generate report: `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out`

## CI/CD & Pre-commit

- **Tests before push**: Run `make test` before pushing to remote. All tests must pass before pushing.
- **CI**: On push/PR to main - tests, lints, vets, builds all apps
- **Release**: On version tags (v1.0.0) - GoReleaser builds releases
- **Pre-commit**: go-fmt, go-vet, go-mod-tidy, golangci-lint, go-test
