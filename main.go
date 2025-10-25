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

type Target struct {
	Name    string     `yaml:"name"`
	Enabled bool       `yaml:"enabled"`
	Notes   string     `yaml:"notes"`
	Paths   []string   `yaml:"paths"`  // measured for size only
	Cmds    [][]string `yaml:"cmds"`   // commands to run when --clean is set
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

func writeStarterConfig(path string) error {
	starter := Config{
		Version: 1,
		Options: Options{DockerPruneByDefault: false},
		Targets: []Target{
			{Name: "docker", Enabled: true, Notes: "Report Docker.raw size; optionally run prunes.", Paths: []string{"~/Library/Caches/docker/*", "~/Library/Caches/buildx/*", "~/Library/Containers/com.docker.docker/Data/vms/0/data/Docker.raw"}},
			{Name: "yarn", Enabled: true, Notes: "Global Yarn cache", Paths: []string{"~/Library/Caches/Yarn/*", "~/.yarn/cache/*"}, Cmds: [][]string{{"yarn", "cache", "clean"}}},
			{Name: "npm", Enabled: true, Notes: "npm cache", Paths: []string{"~/.npm/*"}, Cmds: [][]string{{"npm", "cache", "clean", "--force"}}},
			{Name: "pnpm", Enabled: true, Notes: "pnpm store and cache", Paths: []string{"~/.pnpm-store/*", "~/Library/Caches/pnpm/*"}, Cmds: [][]string{{"pnpm", "store", "prune"}}},
			{Name: "go", Enabled: true, Notes: "Go build & module caches", Paths: []string{"~/Library/Caches/go-build/*", "$GOMODCACHE/cache/*", "$GOPATH/pkg/mod/cache/*"}, Cmds: [][]string{{"go", "clean", "-cache", "-testcache", "-modcache"}}},
			{Name: "maven", Enabled: true, Notes: "Maven local repo purge", Paths: []string{"~/.m2/repository/*"}, Cmds: [][]string{{"mvn", "-q", "dependency:purge-local-repository", "-DreResolve=false"}}},
			{Name: "brew", Enabled: true, Notes: "Homebrew cleanup (old packages, caches)", Paths: []string{"~/Library/Caches/Homebrew/*", "$(brew --cache)/*"}, Cmds: [][]string{{"brew", "cleanup", "-s"}, {"brew", "autoremove"}}},
			{Name: "chrome", Enabled: true, Notes: "No stable CLI for cache clear; informational only.", Paths: []string{"~/Library/Caches/Google/Chrome/*", "~/Library/Application Support/Google/Chrome/*/Cache/*"}},
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

// ----- Main -----

func main() {
	flag.Parse()
	if *flagInit {
		if err := writeStarterConfig(*flagConfig); err != nil {
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

	// run commands (if clean)
	for _, t := range targets {
		if len(t.Cmds) == 0 {
			continue
		}
		if !*flagClean {
			// record would-run
			for _, c := range t.Cmds {
				_, err := exec.LookPath(c[0])
				rep.Commands[t.Name] = append(rep.Commands[t.Name], CmdResult{Cmd: c, Found: err == nil})
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
