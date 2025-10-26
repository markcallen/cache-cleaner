package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ----- CLI flags -----
var (
	flagClean       = flag.Bool("clean", false, "Run safe CLI clean commands (default: dry-run)")
	flagTargets     = flag.String("targets", "all", "Comma-separated targets to scan/clean (or 'all')")
	flagJSON        = flag.Bool("json", false, "Output results as JSON")
	flagConfig      = flag.String("config", defaultConfigPath(), "Path to YAML config")
	flagDockerPrune = flag.Bool("docker-prune", false, "Add docker prune commands at runtime")
	flagInit        = flag.Bool("init", false, "Write a starter config to --config and exit")
	flagForce       = flag.Bool("force", false, "Force overwrite existing config (use with --init)")
	flagListTargets = flag.Bool("list-targets", false, "List all available targets and exit")
	flagCheckTools  = flag.Bool("check-tools", false, "Check if required tools are installed and exit")
	flagDetails     = flag.Bool("details", false, "Show detailed per-directory information")
)

// ----- Config types -----

type Config struct {
	Version int      `yaml:"version"`
	Options Options  `yaml:"options"`
	Targets []Target `yaml:"targets"`
}

type Options struct {
	DockerPruneByDefault bool `yaml:"dockerPruneByDefault"`
}

type Tool struct {
	Name         string `yaml:"name"`          // tool name (e.g., "docker", "npm")
	Version      string `yaml:"version"`       // minimum required version (optional)
	InstallCmd   string `yaml:"installCmd"`   // installation command (defaults to brew if tool exists in brew)
	InstallNotes string `yaml:"installNotes"` // optional installation notes
	CheckPath    string `yaml:"checkPath"`    // optional file path to check for existence instead of using PATH
}

type Target struct {
	Name    string     `yaml:"name"`
	Enabled bool       `yaml:"enabled"`
	Notes   string     `yaml:"notes"`
	Paths   []string   `yaml:"paths"`         // measured for size only
	Cmds    [][]string `yaml:"cmds"`          // commands to run when --clean is set
	Tools   []Tool     `yaml:"tools"`         // required tools for this target
}

// ----- Report types -----

type CmdResult struct {
	Cmd   []string `json:"cmd"`
	Found bool     `json:"found"`
	Error string   `json:"error,omitempty"`
}

type Finding struct {
	Path      string    `json:"path"`
	SizeBytes int64     `json:"size_bytes"`
	Items     int       `json:"items"`
	Err       string    `json:"error,omitempty"`
	ModMax    time.Time `json:"latest_mtime"`
}

type Report struct {
	Hostname string                       `json:"hostname"`
	OS       string                       `json:"os"`
	Arch     string                       `json:"arch"`
	DryRun   bool                         `json:"dry_run"`
	When     time.Time                    `json:"when"`
	Totals   map[string]uint64            `json:"totals_by_target_bytes"`
	Findings map[string][]Finding         `json:"findings"`
	Commands map[string][]CmdResult       `json:"commands"`
	Warnings []string                     `json:"warnings"`
}

// ----- Utilities -----

func defaultConfigPath() string {
	h, _ := os.UserHomeDir()
	if h == "" {
		return "./config.yaml"
	}
	return filepath.Join(h, ".config", "mac-cache-cleaner", "config.yaml")
}

func ensureDir(p string) error { return os.MkdirAll(filepath.Dir(p), 0o755) }

func home() string { h, _ := os.UserHomeDir(); return h }
func expand(p string) string { return os.ExpandEnv(strings.ReplaceAll(p, "~", home())) }

func human(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := float64(n)
	for i, u := range units {
		v /= 1024
		if v < 1024 || i == len(units)-1 {
			return fmt.Sprintf("%.2f %s", v, u)
		}
	}
	return fmt.Sprintf("%.2f TB", v/1024)
}

func inspectPath(root string) (Finding, error) {
	f := Finding{Path: root}
	fi, err := os.Stat(root)
	if err != nil {
		return f, err
	}
	if !fi.IsDir() {
		f.Items = 1
		f.SizeBytes = fi.Size()
		f.ModMax = fi.ModTime()
		return f, nil
	}
	errWalk := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if f.Err == "" {
				f.Err = err.Error()
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return nil
		}
		f.Items++
		f.SizeBytes += info.Size()
		if info.ModTime().After(f.ModMax) {
			f.ModMax = info.ModTime()
		}
		return nil
	})
	return f, errWalk
}

func expandGlobs(pattern string) ([]string, error) {
	pattern = expand(pattern)
	if strings.Contains(pattern, "$(brew --cache)") {
		if p, err := exec.LookPath("brew"); err == nil && p != "" {
			out, err := exec.Command("brew", "--cache").Output()
			if err == nil {
				pattern = strings.ReplaceAll(pattern, "$(brew --cache)", strings.TrimSpace(string(out)))
			} else {
				pattern = strings.ReplaceAll(pattern, "$(brew --cache)", "~/Library/Caches/Homebrew")
			}
		} else {
			pattern = strings.ReplaceAll(pattern, "$(brew --cache)", "~/Library/Caches/Homebrew")
		}
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// ----- Config IO -----

func writeStarterConfig(path string, force bool) error {
	// Check if file exists
	if _, err := os.Stat(path); err == nil {
		if !force {
			return fmt.Errorf("config file already exists at %s. Use --force to overwrite", path)
		}
		// Backup existing file with timestamp
		backupPath := fmt.Sprintf("%s.%s", path, time.Now().Format("20060102-150405"))
		if err := os.Rename(path, backupPath); err != nil {
			return fmt.Errorf("failed to backup existing config: %w", err)
		}
		fmt.Printf("Existing config backed up to: %s\n", backupPath)
	}
	
	starter := Config{
		Version: 1,
		Options: Options{DockerPruneByDefault: false},
		Targets: []Target{
			{Name: "docker", Enabled: true, Notes: "Docker caches and images (safe CLI prune only)", Paths: []string{"~/Library/Caches/docker/*", "~/Library/Caches/buildx/*", "~/Library/Containers/com.docker.docker/Data/vms/0/data/Docker.raw"}, Cmds: [][]string{{"docker", "builder", "prune", "-af"}, {"docker", "system", "prune", "-af", "--volumes"}}, Tools: []Tool{{Name: "docker", InstallCmd: "brew install --cask docker"}}},
			{Name: "brew", Enabled: true, Notes: "Homebrew cleanup (removes old packages and caches)", Paths: []string{"~/Library/Caches/Homebrew/*", "$(brew --cache)/*"}, Cmds: [][]string{{"brew", "cleanup", "-s"}, {"brew", "autoremove"}}, Tools: []Tool{{Name: "brew", InstallCmd: "/bin/bash -c \"$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\""}}},
			{Name: "npm", Enabled: true, Notes: "npm cache", Paths: []string{"~/.npm/*"}, Cmds: [][]string{{"npm", "cache", "clean", "--force"}}, Tools: []Tool{{Name: "npm", InstallCmd: "brew install node"}}},
			{Name: "yarn", Enabled: true, Notes: "Global Yarn cache", Paths: []string{"~/Library/Caches/Yarn/*", "~/.yarn/cache/*"}, Cmds: [][]string{{"yarn", "cache", "clean"}}, Tools: []Tool{{Name: "yarn", InstallCmd: "brew install yarn"}}},
			{Name: "pnpm", Enabled: true, Notes: "pnpm store and cache", Paths: []string{"~/.pnpm-store/*", "~/Library/Caches/pnpm/*"}, Cmds: [][]string{{"pnpm", "store", "prune"}}, Tools: []Tool{{Name: "pnpm", InstallCmd: "brew install pnpm"}}},
			{Name: "node-versions", Enabled: true, Notes: "Node version managers (nvm, volta)", Paths: []string{"~/.nvm/versions/node/*", "~/.volta/tools/image/node/*"}, Cmds: [][]string{{"nvm", "cache", "clear"}, {"volta", "clean"}}, Tools: []Tool{{Name: "nvm", InstallCmd: "curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.3/install.sh | bash", InstallNotes: "After installation, restart your terminal or run: source ~/.bashrc or source ~/.zshrc", CheckPath: "~/.nvm/nvm.sh"}, {Name: "volta", InstallCmd: "curl https://get.volta.sh | bash"}}},
			{Name: "expo", Enabled: true, Notes: "Expo and React Native caches", Paths: []string{"~/.expo/*", "~/.cache/expo/*"}, Cmds: [][]string{{"expo", "start", "-c"}}, Tools: []Tool{{Name: "expo", InstallCmd: "npm install -g expo-cli"}}},
			{Name: "go", Enabled: true, Notes: "Go build & module caches", Paths: []string{"~/Library/Caches/go-build/*", "$GOMODCACHE/cache/*", "$GOPATH/pkg/mod/cache/*"}, Cmds: [][]string{{"go", "clean", "-cache", "-testcache", "-modcache"}}, Tools: []Tool{{Name: "go", InstallCmd: "brew install go"}}},
			{Name: "rust", Enabled: true, Notes: "Rust registry and build caches (requires cargo-cache: cargo install cargo-cache)", Paths: []string{"~/.cargo/registry/*", "~/.cargo/git/*"}, Cmds: [][]string{{"cargo", "cache", "-a"}}, Tools: []Tool{{Name: "cargo", InstallCmd: "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"}, {Name: "cargo-cache", InstallCmd: "cargo install cargo-cache", InstallNotes: "Install this after cargo is installed"}}},
			{Name: "python", Enabled: true, Notes: "pip, pipenv, and poetry caches", Paths: []string{"~/.cache/pip/*", "~/Library/Caches/pip/*", "~/.local/share/virtualenvs/*", "~/Library/Caches/pypoetry/*"}, Cmds: [][]string{{"pip", "cache", "purge"}, {"poetry", "cache", "clear", "--all", "pypi"}}, Tools: []Tool{{Name: "pip", InstallCmd: "brew install python", InstallNotes: "pip is included with Python installation"}, {Name: "poetry", InstallCmd: "brew install poetry"}}},
			{Name: "conda", Enabled: true, Notes: "Conda package and cache cleanup", Paths: []string{"~/.conda/pkgs/*", "~/.conda/envs/*"}, Cmds: [][]string{{"conda", "clean", "-a", "-y"}}, Tools: []Tool{{Name: "conda", InstallCmd: "brew install miniconda"}}},
			{Name: "maven", Enabled: true, Notes: "Maven local repo purge (safe via plugin)", Paths: []string{"~/.m2/repository/*"}, Cmds: [][]string{{"mvn", "-q", "dependency:purge-local-repository", "-DreResolve=false"}}, Tools: []Tool{{Name: "mvn", InstallCmd: "brew install maven"}}},
			{Name: "gradle", Enabled: true, Notes: "Gradle build caches and wrappers", Paths: []string{"~/.gradle/caches/*", "~/.gradle/wrapper/dists/*"}, Cmds: [][]string{{"gradle", "--stop"}}, Tools: []Tool{{Name: "gradle", InstallCmd: "brew install gradle"}}},
			{Name: "xcode", Enabled: true, Notes: "Xcode build artifacts and caches", Paths: []string{"~/Library/Developer/Xcode/DerivedData/*", "~/Library/Developer/Xcode/Archives/*", "~/Library/Developer/Xcode/ModuleCache.noindex/*"}, Cmds: [][]string{{"xcrun", "simctl", "delete", "unavailable"}}},
			{Name: "ruby", Enabled: true, Notes: "Ruby and Bundler caches", Paths: []string{"~/.gem/cache/*", "~/.bundle/cache/*"}, Cmds: [][]string{{"gem", "cleanup"}, {"bundle", "clean", "--force"}}, Tools: []Tool{{Name: "gem", InstallCmd: "brew install ruby", InstallNotes: "gem is included with Ruby installation"}, {Name: "bundle", InstallCmd: "gem install bundler"}}},
			{Name: "php", Enabled: true, Notes: "Composer PHP cache", Paths: []string{"~/.composer/cache/*"}, Cmds: [][]string{{"composer", "clear-cache"}}, Tools: []Tool{{Name: "composer", InstallCmd: "brew install composer"}}},
			{Name: "dotnet", Enabled: true, Notes: ".NET SDK and NuGet caches", Paths: []string{"~/.nuget/packages/*", "~/.dotnet/tools/*"}, Cmds: [][]string{{"dotnet", "nuget", "locals", "all", "--clear"}}, Tools: []Tool{{Name: "dotnet", InstallCmd: "brew install --cask dotnet"}}},
			{Name: "vscode", Enabled: true, Notes: "VS Code caches and logs", Paths: []string{"~/Library/Application Support/Code/Cache/*", "~/Library/Application Support/Code/CachedData/*", "~/Library/Application Support/Code/GPUCache/*", "~/Library/Application Support/Code/logs/*"}, Cmds: [][]string{}},
			{Name: "jetbrains", Enabled: true, Notes: "JetBrains IDE caches (IntelliJ, PyCharm, WebStorm, etc.)", Paths: []string{"~/Library/Caches/JetBrains/*", "~/Library/Logs/JetBrains/*", "~/Library/Application Support/JetBrains/*/system/caches/*"}, Cmds: [][]string{}},
			{Name: "build-tools", Enabled: true, Notes: "Compiler and build caches (ccache, bazel, Xcode)", Paths: []string{"~/.ccache/*", "~/.bazel-cache/*", "~/.cache/bazel/*"}, Cmds: [][]string{{"ccache", "-C"}}, Tools: []Tool{{Name: "ccache", InstallCmd: "brew install ccache"}}},
			{Name: "chrome", Enabled: true, Notes: "Chrome cache (informational only)", Paths: []string{"~/Library/Caches/Google/Chrome/*", "~/Library/Application Support/Google/Chrome/*/Cache/*"}, Cmds: [][]string{}},
			{Name: "macos", Enabled: false, Notes: "macOS system caches (advanced users only)", Paths: []string{"~/Library/Caches/*", "~/Library/Containers/com.apple.QuickLook.thumbnailcache/*"}, Cmds: [][]string{{"qlmanage", "-r", "cache"}}},
			{Name: "flutter", Enabled: true, Notes: "Flutter and Dart caches (pub, SDK, and analysis artifacts)", Paths: []string{"~/.pub-cache/*", "~/.dartServer/*", "~/Library/Developer/flutter/*", "~/Library/Caches/flutter/*"}, Cmds: [][]string{{"flutter", "pub", "cache", "clean"}}, Tools: []Tool{{Name: "flutter", InstallCmd: "brew install --cask flutter"}}},
			{Name: "android", Enabled: true, Notes: "Android SDK and emulator caches", Paths: []string{"~/.android/cache/*", "~/.android/avd/*", "~/Library/Android/sdk/*"}, Cmds: [][]string{{"sdkmanager", "--update"}}},
			{Name: "android-studio", Enabled: true, Notes: "Android Studio IDE caches, logs, and indexes", Paths: []string{"~/Library/Caches/Google/AndroidStudio*/*", "~/Library/Logs/Google/AndroidStudio*/*", "~/Library/Application Support/Google/AndroidStudio*/system/caches/*", "~/Library/Application Support/Google/AndroidStudio*/system/index/*"}, Cmds: [][]string{}},
		},
	}
	if err := ensureDir(path); err != nil {
		return err
	}
	b, err := yaml.Marshal(starter)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func loadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ----- Tool checking -----

// checkTool checks if a tool is installed and in PATH
func checkTool(tool Tool) (bool, string, error) {
	// If CheckPath is specified, check for file existence instead of PATH
	if tool.CheckPath != "" {
		checkPath := expand(tool.CheckPath)
		if _, err := os.Stat(checkPath); err == nil {
			return true, checkPath, nil
		}
		return false, "", nil
	}
	
	// Check if tool is in PATH
	path, err := exec.LookPath(tool.Name)
	if err != nil {
		return false, "", nil // not found, but not an error
	}
	if path == "" {
		return false, "", nil
	}

	// If version check is required, verify version
	if tool.Version != "" {
		cmd := exec.Command(tool.Name, "--version")
		output, err := cmd.Output()
		if err != nil {
			return false, "", fmt.Errorf("failed to check version: %w", err)
		}
		// Parse version from output (basic check)
		versionStr := strings.TrimSpace(string(output))
		// For now, we just check if version is present - full semantic version comparison would be more complex
		if !strings.Contains(versionStr, tool.Version) && tool.Version != "" {
			return false, versionStr, fmt.Errorf("version mismatch: need %s, found version info: %s", tool.Version, versionStr)
		}
		return true, versionStr, nil
	}

	return true, path, nil
}

// getInstallCommand returns the installation command for a tool
func getInstallCommand(tool Tool) string {
	// If install command is explicitly provided, use it
	if tool.InstallCmd != "" {
		return tool.InstallCmd
	}

	// Default to brew if available
	if _, err := exec.LookPath("brew"); err == nil {
		// Check if tool is available in brew
		cmd := exec.Command("brew", "search", tool.Name)
		output, _ := cmd.Output()
		if strings.Contains(strings.ToLower(string(output)), tool.Name) {
			return fmt.Sprintf("brew install %s", tool.Name)
		}
	}

	// Fallback to generic message
	return fmt.Sprintf("Install %s using your preferred package manager", tool.Name)
}

// ----- Command runner -----

func runCmd(cmd []string) CmdResult {
	res := CmdResult{Cmd: cmd}
	if len(cmd) == 0 {
		res.Error = "empty command"
		return res
	}
	if _, err := exec.LookPath(cmd[0]); err != nil {
		res.Found = false
		res.Error = "not found"
		return res
	}
	res.Found = true
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		res.Error = err.Error()
	}
	return res
}

// ----- Tool checker -----

func checkTools(cfg *Config) {
	// Filter by --targets if specified
	sel := map[string]bool{}
	for _, t := range strings.Split(*flagTargets, ",") {
		if s := strings.TrimSpace(strings.ToLower(t)); s != "" {
			sel[s] = true
		}
	}
	
	// Collect all unique tools from selected targets
	toolMap := make(map[string]map[string]bool) // tool name -> map[targetName]bool
	
	for _, target := range cfg.Targets {
		if !target.Enabled {
			continue
		}
		// Filter by selected targets
		if !(sel["all"] || sel[strings.ToLower(target.Name)]) {
			continue
		}
		for _, tool := range target.Tools {
			if toolMap[tool.Name] == nil {
				toolMap[tool.Name] = make(map[string]bool)
			}
			toolMap[tool.Name][target.Name] = true
		}
	}
	
	if len(toolMap) == 0 {
		fmt.Println("No tool requirements defined in targets.")
		return
	}
	
	fmt.Printf("Tool Status Check (config: %s)\n\n", *flagConfig)
	
	var allOK = true
	// Sort tool names for consistent output
	var sortedNames []string
	for name := range toolMap {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)
	
	for _, name := range sortedNames {
		// Find the first occurrence of this tool to get its config
		tool := Tool{Name: name}
		found := false
		for _, target := range cfg.Targets {
			for _, t := range target.Tools {
				if t.Name == name {
					tool = t
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		
		installed, info, err := checkTool(tool)
		targets := make([]string, 0, len(toolMap[name]))
		for t := range toolMap[name] {
			targets = append(targets, t)
		}
		sort.Strings(targets)
		
		if installed {
			versionInfo := ""
			if info != "" && !strings.Contains(info, "/") {
				// If info is not a path, treat it as version info
				versionInfo = fmt.Sprintf(" (%s)", info)
			}
			fmt.Printf("✓ %s [OK]%s\n", name, versionInfo)
		} else {
			allOK = false
			installCmd := getInstallCommand(tool)
			fmt.Printf("✗ %s [MISSING]\n", name)
			fmt.Printf("  Required by targets: %s\n", strings.Join(targets, ", "))
			fmt.Printf("  Install: %s", installCmd)
			if tool.InstallNotes != "" {
				fmt.Printf(" - %s", tool.InstallNotes)
			}
			fmt.Println()
			if err != nil {
				fmt.Printf("  Error: %v\n", err)
			}
		}
		fmt.Println()
	}
	
	if allOK {
		fmt.Println("All required tools are installed!")
		os.Exit(0)
	} else {
		fmt.Println("Some required tools are missing. See installation commands above.")
		os.Exit(1)
	}
}

// ----- Main -----

func main() {
	flag.Parse()
	if *flagListTargets {
		cfg, err := loadConfig(*flagConfig)
		if err != nil {
			fmt.Println("config error:", err)
			fmt.Println("Tip: run with --init to create a starter config")
			os.Exit(1)
		}
		fmt.Printf("Available targets in %s:\n\n", *flagConfig)
		for _, t := range cfg.Targets {
			status := "[DISABLED]"
			if t.Enabled {
				status = "[ENABLED] "
			}
			fmt.Printf("%s %s\n", status, t.Name)
			if t.Notes != "" {
				fmt.Printf("        %s\n", t.Notes)
			}
			fmt.Println()
		}
		return
	}

	if *flagInit {
		if err := writeStarterConfig(*flagConfig, *flagForce); err != nil {
			fmt.Println("init error:", err)
			os.Exit(1)
		}
		fmt.Println("Starter config written to:", *flagConfig)
		return
	}
	
	cfg, err := loadConfig(*flagConfig)
	if err != nil {
		fmt.Println("config error:", err)
		fmt.Println("Tip: run with --init to create a starter config")
		os.Exit(1)
	}
	
	if *flagCheckTools {
		checkTools(cfg)
		return
	}

	rep := Report{OS: runtime.GOOS, Arch: runtime.GOARCH, DryRun: !*flagClean, When: time.Now(), Totals: map[string]uint64{}, Findings: map[string][]Finding{}, Commands: map[string][]CmdResult{}, Warnings: []string{}}
	if h, _ := os.Hostname(); h != "" {
		rep.Hostname = h
	}

	// inject docker prune if configured or flagged
	if cfg.Options.DockerPruneByDefault || *flagDockerPrune {
		for i := range cfg.Targets {
			if cfg.Targets[i].Name == "docker" {
				cfg.Targets[i].Cmds = append(cfg.Targets[i].Cmds, []string{"docker", "builder", "prune", "-af"}, []string{"docker", "system", "prune", "-af", "--volumes"})
			}
		}
	}

	// filter by --targets
	sel := map[string]bool{}
	for _, t := range strings.Split(*flagTargets, ",") {
		if s := strings.TrimSpace(strings.ToLower(t)); s != "" {
			sel[s] = true
		}
	}

	var targets []Target
	for _, t := range cfg.Targets {
		if !t.Enabled {
			continue
		}
		if !(sel["all"] || sel[strings.ToLower(t.Name)]) {
			continue
		}
		targets = append(targets, t)
	}
	if len(targets) == 0 {
		fmt.Println("No targets selected.")
		os.Exit(0)
	}

	// measure sizes
	for _, t := range targets {
		var sum int64
		var expanded []string
		for _, p := range t.Paths {
			matches, err := expandGlobs(p)
			if err != nil {
				rep.Warnings = append(rep.Warnings, fmt.Sprintf("glob error %s:%s: %v", t.Name, p, err))
				continue
			}
			expanded = append(expanded, matches...)
		}
		if len(expanded) == 0 {
			rep.Findings[t.Name] = []Finding{{Path: "(none)"}}
		}
		for _, p := range expanded {
			f, err := inspectPath(p)
			if err != nil {
				f.Err = err.Error()
			}
			rep.Findings[t.Name] = append(rep.Findings[t.Name], f)
			sum += f.SizeBytes
		}
		rep.Totals[t.Name] = uint64(sum)
	}

	// check tools and run commands (if clean)
	for _, t := range targets {
		// Check tool requirements first - create a map of tool name -> installed status
		toolStatus := make(map[string]bool) // Track which tools are installed
		missingTools := make(map[string]bool) // Track which tools are missing
		if len(t.Tools) > 0 {
			for _, tool := range t.Tools {
				installed, _, err := checkTool(tool)
				toolStatus[tool.Name] = installed
				if !installed {
					missingTools[tool.Name] = true
					installCmd := getInstallCommand(tool)
					errMsg := fmt.Sprintf("Required tool '%s' is not installed or not in PATH", tool.Name)
					if err != nil {
						errMsg = fmt.Sprintf("Required tool '%s' version check failed: %v", tool.Name, err)
					}
					installGuidance := fmt.Sprintf("To install: %s", installCmd)
					if tool.InstallNotes != "" {
						installGuidance += " - " + tool.InstallNotes
					}
					rep.Warnings = append(rep.Warnings, fmt.Sprintf("[%s] %s. %s", t.Name, errMsg, installGuidance))
				}
			}
		}

		if len(t.Cmds) == 0 {
			continue
		}
		if !*flagClean {
			// record would-run
			for _, c := range t.Cmds {
				found := false
				errorMsg := ""
				
				if len(c) > 0 {
					// Check if this is a known tool in our tools list
					if installed, ok := toolStatus[c[0]]; ok {
						// Use the tool's installed status
						found = installed
						if !installed {
							errorMsg = fmt.Sprintf("tool '%s' not installed", c[0])
						}
					} else {
						// Not a tool in our list, just check PATH
						_, err := exec.LookPath(c[0])
						found = err == nil
						if !found {
							errorMsg = "not found"
						}
					}
				}
				
				rep.Commands[t.Name] = append(rep.Commands[t.Name], CmdResult{
					Cmd:   c,
					Found: found,
					Error: errorMsg,
				})
			}
			continue
		}
		for _, c := range t.Cmds {
			rep.Commands[t.Name] = append(rep.Commands[t.Name], runCmd(c))
		}
	}

	// output
	if *flagJSON {
		b, _ := json.MarshalIndent(rep, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Printf("Config: %s\nScan: %s\nDry-run: %v\n\n", *flagConfig, rep.When.Format(time.RFC3339), rep.DryRun)
	// sort targets by size desc
	type kv struct{ k string; v uint64 }
	var list []kv
	for k, v := range rep.Totals {
		list = append(list, kv{k, v})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].v > list[j].v })
	for _, e := range list {
		fmt.Printf("[%s] %s\n", e.k, human(int64(e.v)))
		if *flagDetails {
			for _, f := range rep.Findings[e.k] {
				if f.Path == "(none)" {
					fmt.Println("  - no paths found")
					continue
				}
				status := ""
				if f.Err != "" {
					status = " (" + f.Err + ")"
				}
				fmt.Printf("  - %s — %s, %d items, latest %s%s\n", f.Path, human(f.SizeBytes), f.Items, f.ModMax.Format(time.RFC3339), status)
			}
		}
		if cr, ok := rep.Commands[e.k]; ok && len(cr) > 0 {
			fmt.Println("  Commands:")
			for _, c := range cr {
				found := "missing"
				if c.Found {
					found = "ok"
				}
				err := ""
				if c.Error != "" {
					err = " (" + c.Error + ")"
				}
				fmt.Printf("    - %s [%s]%s\n", strings.Join(c.Cmd, " "), found, err)
			}
		}
		fmt.Println()
	}
	if len(rep.Warnings) > 0 {
		fmt.Println("Warnings:")
		for _, w := range rep.Warnings {
			fmt.Println(" -", w)
		}
	}
}
