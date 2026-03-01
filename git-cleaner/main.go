package main

import (
	"bufio"
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

	"github.com/olekukonko/tablewriter"
	yaml "gopkg.in/yaml.v3"
)

// ----- Version info -----
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// ----- CLI flags -----
var (
	flagScan    = flag.String("scan", "", "Directory to scan for .git directories (overrides config default)")
	flagClean   = flag.Bool("clean", false, "Run git gc in each repository and show disk savings")
	flagJSON    = flag.Bool("json", false, "Output results as JSON")
	flagConfig  = flag.String("config", defaultConfigPath(), "Path to YAML config")
	flagInit    = flag.Bool("init", false, "Write a starter config to --config and exit")
	flagForce   = flag.Bool("force", false, "Force overwrite existing config (use with --init)")
	flagYes     = flag.Bool("yes", false, "Skip confirmation prompt for cleanup")
	flagShowPct = flag.Bool("show-pct", false, "Show .git size as percentage of repository size")
)

// ----- Config types -----

type Config struct {
	Version int     `yaml:"version"`
	Options Options `yaml:"options"`
}

type Options struct {
	DefaultScanPath string `yaml:"defaultScanPath"`
}

// ----- Finding/report types -----

type Finding struct {
	Path           string  `json:"path"`
	RepoPath       string  `json:"repo_path"`
	SizeBytes      int64   `json:"size_bytes"`
	Items          int     `json:"items"`
	RepoTotalBytes int64   `json:"repo_total_bytes,omitempty"`
	GitPercent     float64 `json:"git_percent,omitempty"`
}

type Report struct {
	Hostname           string    `json:"hostname,omitempty"`
	OS                 string    `json:"os"`
	Arch               string    `json:"arch"`
	When               time.Time `json:"when"`
	ScanPath           string    `json:"scan_path"`
	DryRun             bool      `json:"dry_run"`
	ShowPct            bool      `json:"show_pct"`
	TotalBytes         int64     `json:"total_bytes"`
	TotalAfterBytes    int64     `json:"total_after_bytes,omitempty"`
	DiskSavingsBytes   int64     `json:"disk_savings_bytes,omitempty"`
	DiskSavingsPercent float64   `json:"disk_savings_percent,omitempty"`
	CleanedRepos       int       `json:"cleaned_repos,omitempty"`
	Findings           []Finding `json:"findings"`
	Warnings           []string  `json:"warnings,omitempty"`
}

func defaultConfigPath() string {
	h, _ := os.UserHomeDir()
	if h == "" {
		return "./config.yaml"
	}
	return filepath.Join(h, ".config", "git-cleaner", "config.yaml")
}

func ensureDir(p string) error { return os.MkdirAll(filepath.Dir(p), 0o755) }

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

func checkVersionFlag() bool {
	for _, arg := range os.Args[1:] {
		if arg == "-version" || arg == "--version" {
			return true
		}
	}
	return false
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
		return f, nil
	}
	errWalk := filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
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
		return nil
	})
	return f, errWalk
}

func scanDirectory(root string) []Finding {
	var findings []Finding

	root = filepath.Clean(root)
	rootAbs, err := filepath.Abs(root)
	if err == nil {
		root = rootAbs
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() == ".git" {
			repoPath := filepath.Dir(path)
			f, err := inspectPath(path)
			if err != nil {
				return nil
			}
			f.RepoPath = repoPath
			findings = append(findings, f)
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: error walking directory: %v\n", err)
	}

	return findings
}

func enrichWithRepoPercent(findings []Finding) []string {
	var warnings []string
	for i := range findings {
		repo, err := inspectPath(findings[i].RepoPath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("failed repo size for %s: %v", findings[i].RepoPath, err))
			continue
		}
		findings[i].RepoTotalBytes = repo.SizeBytes
		if repo.SizeBytes > 0 {
			findings[i].GitPercent = (float64(findings[i].SizeBytes) / float64(repo.SizeBytes)) * 100
		}
	}
	return warnings
}

func runGitGC(repoPath string, jsonMode bool) error {
	cmd := exec.Command("git", "gc")
	cmd.Dir = repoPath
	if jsonMode {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

func displayResults(findings []Finding, total int64, showPct bool) {
	if len(findings) == 0 {
		fmt.Println("No .git directories found.")
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	if showPct {
		table.Header("Repository Path", ".git Size", "Items", "Git %")
	} else {
		table.Header("Repository Path", ".git Size", "Items")
	}

	sortedFindings := make([]Finding, len(findings))
	copy(sortedFindings, findings)
	sort.Slice(sortedFindings, func(i, j int) bool {
		return sortedFindings[i].SizeBytes > sortedFindings[j].SizeBytes
	})

	var totalItems int
	for _, f := range sortedFindings {
		if showPct {
			if err := table.Append(f.RepoPath, human(f.SizeBytes), fmt.Sprintf("%d", f.Items), fmt.Sprintf("%.2f%%", f.GitPercent)); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: error appending to table: %v\n", err)
			}
		} else {
			if err := table.Append(f.RepoPath, human(f.SizeBytes), fmt.Sprintf("%d", f.Items)); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: error appending to table: %v\n", err)
			}
		}
		totalItems += f.Items
	}

	if showPct {
		table.Footer("TOTAL", human(total), fmt.Sprintf("%d", totalItems), "")
	} else {
		table.Footer("TOTAL", human(total), fmt.Sprintf("%d", totalItems))
	}
	if err := table.Render(); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering table: %v\n", err)
	}
}

func expandScanPath(scanPath string) (string, error) {
	if scanPath == "" {
		return "", fmt.Errorf("scan path is required")
	}
	if strings.HasPrefix(scanPath, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			if len(scanPath) > 1 && scanPath[1] == filepath.Separator {
				scanPath = filepath.Join(home, scanPath[2:])
			} else if len(scanPath) == 1 {
				scanPath = home
			} else {
				scanPath = filepath.Join(home, scanPath[1:])
			}
		}
	}
	scanPath = os.ExpandEnv(scanPath)
	abs, err := filepath.Abs(scanPath)
	if err != nil {
		return "", fmt.Errorf("invalid scan path: %w", err)
	}
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		return "", fmt.Errorf("scan path does not exist: %s", abs)
	}
	return abs, nil
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

func writeStarterConfig(path string, force bool) error {
	if _, err := os.Stat(path); err == nil {
		if !force {
			return fmt.Errorf("config file already exists at %s. Use --force to overwrite", path)
		}
		backupPath := fmt.Sprintf("%s.%s", path, time.Now().Format("20060102-150405"))
		if err := os.Rename(path, backupPath); err != nil {
			return fmt.Errorf("failed to backup existing config: %w", err)
		}
		fmt.Printf("Existing config backed up to: %s\n", backupPath)
	}

	starter := Config{
		Version: 1,
		Options: Options{DefaultScanPath: "~/src"},
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

func resolveScanPath(flagValue string, cfg *Config) (string, error) {
	if flagValue != "" {
		return expandScanPath(flagValue)
	}
	if cfg != nil && cfg.Options.DefaultScanPath != "" {
		return expandScanPath(cfg.Options.DefaultScanPath)
	}
	return "", fmt.Errorf("scan path is required")
}

func confirmCleanup() bool {
	fmt.Print("Proceed with git gc for all discovered repositories? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}

func main() {
	if checkVersionFlag() {
		fmt.Printf("version %s, commit %s, built at %s\n", version, commit, date)
		return
	}

	flag.Parse()

	if *flagInit {
		if err := writeStarterConfig(*flagConfig, *flagForce); err != nil {
			fmt.Println("init error:", err)
			os.Exit(1)
		}
		fmt.Println("Starter config written to:", *flagConfig)
		return
	}

	var cfg *Config
	if loaded, err := loadConfig(*flagConfig); err == nil {
		cfg = loaded
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config %s: %v\n", *flagConfig, err)
	}

	scanPath, err := resolveScanPath(*flagScan, cfg)
	if err != nil {
		if err.Error() == "scan path is required" {
			fmt.Println("Error: --scan flag is required (or configure options.defaultScanPath)")
			fmt.Println("Usage: git-cleaner --scan <directory> [--clean] [--json]")
		} else {
			fmt.Printf("Error: %v\n", err)
		}
		os.Exit(1)
	}

	findings := scanDirectory(scanPath)
	if *flagShowPct {
		_ = enrichWithRepoPercent(findings)
	}

	var totalBefore int64
	for _, f := range findings {
		totalBefore += f.SizeBytes
	}

	report := Report{
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		When:       time.Now(),
		ScanPath:   scanPath,
		DryRun:     !*flagClean,
		ShowPct:    *flagShowPct,
		TotalBytes: totalBefore,
		Findings:   findings,
	}
	if h, err := os.Hostname(); err == nil {
		report.Hostname = h
	}

	if len(findings) == 0 {
		if *flagJSON {
			b, _ := json.MarshalIndent(report, "", "  ")
			fmt.Println(string(b))
			return
		}
		fmt.Println("No .git directories found.")
		return
	}

	if *flagJSON {
		if *flagClean {
			if !*flagYes {
				report.Warnings = append(report.Warnings, "cleanup skipped: confirmation required (use --yes with --clean in non-interactive mode)")
				b, _ := json.MarshalIndent(report, "", "  ")
				fmt.Println(string(b))
				return
			}

			var cleanedCount int
			for _, f := range findings {
				if err := runGitGC(f.RepoPath, true); err != nil {
					report.Warnings = append(report.Warnings, fmt.Sprintf("%s: %v", f.RepoPath, err))
					continue
				}
				cleanedCount++
			}

			afterFindings := scanDirectory(scanPath)
			if *flagShowPct {
				_ = enrichWithRepoPercent(afterFindings)
			}
			var totalAfter int64
			for _, f := range afterFindings {
				totalAfter += f.SizeBytes
			}
			report.TotalAfterBytes = totalAfter
			report.DiskSavingsBytes = totalBefore - totalAfter
			report.CleanedRepos = cleanedCount
			report.Findings = afterFindings
			if totalBefore > 0 {
				report.DiskSavingsPercent = float64(report.DiskSavingsBytes) / float64(totalBefore) * 100
			}
		}

		b, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(b))
		return
	}

	fmt.Printf("Scanning %s for .git directories...\n", scanPath)
	fmt.Printf("\nFound %d repositories:\n\n", len(findings))
	displayResults(findings, totalBefore, *flagShowPct)

	if !*flagClean {
		return
	}

	if !*flagYes && !confirmCleanup() {
		fmt.Println("Cleanup cancelled.")
		return
	}

	fmt.Printf("\nRunning git gc in %d repositories...\n", len(findings))
	var errors []string
	var cleanedCount int

	for _, f := range findings {
		fmt.Printf("  Cleaning %s...\n", f.RepoPath)
		if err := runGitGC(f.RepoPath, false); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", f.RepoPath, err))
			continue
		}
		cleanedCount++
	}

	if len(errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range errors {
			fmt.Printf("  - %s\n", e)
		}
	}

	fmt.Println("\nRescanning after cleanup...")
	time.Sleep(100 * time.Millisecond)
	afterFindings := scanDirectory(scanPath)
	if *flagShowPct {
		_ = enrichWithRepoPercent(afterFindings)
	}

	var totalAfter int64
	for _, f := range afterFindings {
		totalAfter += f.SizeBytes
	}

	diskSavings := totalBefore - totalAfter
	fmt.Printf("\nResults after cleanup:\n\n")
	displayResults(afterFindings, totalAfter, *flagShowPct)

	pct := 0.0
	if totalBefore > 0 {
		pct = float64(diskSavings) / float64(totalBefore) * 100
	}
	fmt.Printf("\nDisk savings: %s (%.2f%%)\n", human(diskSavings), pct)
	fmt.Printf("Cleaned %d repositories\n", cleanedCount)
}
