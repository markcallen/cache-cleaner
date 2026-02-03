package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
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
	flagClean  = flag.Bool("clean", false, "Delete found cache directories")
	flagYes    = flag.Bool("yes", false, "Skip confirmation prompt for cleanup")
	flagJSON   = flag.Bool("json", false, "Output results as JSON")
	flagConfig = flag.String("config", defaultConfigPath(), "Path to YAML config")
	flagInit   = flag.Bool("init", false, "Write a starter config to --config and exit")
	flagForce  = flag.Bool("force", false, "Force overwrite existing config (use with --init)")
	flagScan   = flag.String("scan", "", "Directory to scan (overrides config default)")
	flagDepth  = flag.Int("depth", 0, "Max scan depth (0 = use config default, overrides config)")
	flagLangs  = flag.String("languages", "", "Comma-separated list of languages to scan")
)

// ----- Config types -----

type Config struct {
	Version   int        `yaml:"version"`
	Options   Options    `yaml:"options"`
	Languages []Language `yaml:"languages"`
}

type Options struct {
	DefaultScanPath string `yaml:"defaultScanPath"`
	MaxDepth        int    `yaml:"maxDepth"`
	DetectLanguage  bool   `yaml:"detectLanguage"` // If true, detect language per directory and search only relevant patterns
}

type Language struct {
	Name       string   `yaml:"name"`
	Enabled    bool     `yaml:"enabled"`
	Priority   int      `yaml:"priority"` // Detection priority (lower number = higher priority, default: 5)
	Patterns   []string `yaml:"patterns"`
	Signatures []string `yaml:"signatures"` // Files/directories that indicate this language (e.g., "package.json" for node)
}

// ----- Finding types -----

type Finding struct {
	Path        string    `json:"path"`
	ProjectRoot string    `json:"project_root,omitempty"` // Directory where language was detected
	SizeBytes   int64     `json:"size_bytes"`
	Items       int       `json:"items"`
	Pattern     string    `json:"pattern"`
	Language    string    `json:"language"`
	Err         string    `json:"error,omitempty"`
	ModMax      time.Time `json:"latest_mtime"`
}

type Report struct {
	Hostname string    `json:"hostname"`
	OS       string    `json:"os"`
	Arch     string    `json:"arch"`
	DryRun   bool      `json:"dry_run"`
	When     time.Time `json:"when"`
	ScanPath string    `json:"scan_path"`
	MaxDepth int       `json:"max_depth"`
	Total    int64     `json:"total_bytes"`
	Findings []Finding `json:"findings"`
	Warnings []string  `json:"warnings"`
}

// ----- Utilities -----

func defaultConfigPath() string {
	h, _ := os.UserHomeDir()
	if h == "" {
		return "./config.yaml"
	}
	return filepath.Join(h, ".config", "dev-cache", "config.yaml")
}

func ensureDir(p string) error { return os.MkdirAll(filepath.Dir(p), 0o755) }

func home() string           { h, _ := os.UserHomeDir(); return h }
func expand(p string) string { return os.ExpandEnv(strings.ReplaceAll(p, "~", home())) }

func checkVersionFlag() bool {
	for _, arg := range os.Args[1:] {
		if arg == "-version" || arg == "--version" {
			return true
		}
	}
	return false
}

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

// isCacheDirectory returns true if the finding represents a cache directory (has a non-empty pattern)
func isCacheDirectory(f Finding) bool {
	return f.Pattern != ""
}

// getLanguageForExclusion returns the appropriate language string based on detected language and exclusion status.
// Excluded directories should have empty language (not "no language found").
// If not excluded and no language detected, returns "no language found".
// Otherwise returns the detected language.
func getLanguageForExclusion(detectedLang string, isExcluded bool) string {
	if isExcluded {
		return ""
	}
	if detectedLang == "" {
		return "no language found"
	}
	return detectedLang
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
		Options: Options{
			DefaultScanPath: "~/src",
			MaxDepth:        1,
			DetectLanguage:  true,
		},
		Languages: []Language{
			{Name: "node", Enabled: true, Priority: 10, Patterns: []string{"node_modules", ".npm", ".yarn", ".pnpm-store"}, Signatures: []string{"package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml"}},
			{Name: "python", Enabled: true, Priority: 5, Patterns: []string{".venv", "venv", "__pycache__", ".pytest_cache", ".mypy_cache", ".tox"}, Signatures: []string{"requirements.txt", "setup.py", "pyproject.toml", "Pipfile", "setup.cfg"}},
			{Name: "go", Enabled: true, Priority: 5, Patterns: []string{"vendor"}, Signatures: []string{"go.mod", "go.sum", "Gopkg.toml"}},
			{Name: "rust", Enabled: true, Priority: 5, Patterns: []string{"target"}, Signatures: []string{"Cargo.toml", "Cargo.lock"}},
			{Name: "java", Enabled: true, Priority: 5, Patterns: []string{"target", ".gradle", "build"}, Signatures: []string{"pom.xml", "build.gradle", "build.gradle.kts", ".mvn"}},
			{Name: "nextjs", Enabled: true, Priority: 1, Patterns: []string{"node_modules", ".npm", ".yarn", ".pnpm-store", ".next", "dist", "build", "out", ".cache"}, Signatures: []string{"next.config.js", "next.config.ts", "next.config.mjs"}},
			{Name: "vue", Enabled: true, Priority: 2, Patterns: []string{"node_modules", ".npm", ".yarn", ".pnpm-store", ".nuxt", "dist", "build", "out", ".cache", ".parcel-cache"}, Signatures: []string{"nuxt.config.js", "nuxt.config.ts", "nuxt.config.mjs", "vue.config.js"}},
			{Name: "php", Enabled: true, Priority: 5, Patterns: []string{"vendor"}, Signatures: []string{"composer.json", "composer.lock"}},
			{Name: "ruby", Enabled: true, Priority: 5, Patterns: []string{"vendor/bundle"}, Signatures: []string{"Gemfile", "Gemfile.lock", "Rakefile"}},
			{Name: "dotnet", Enabled: true, Priority: 5, Patterns: []string{"bin", "obj"}, Signatures: []string{"*.csproj", "*.sln", "*.fsproj", "*.vbproj", "project.json"}},
			{Name: "cpp", Enabled: true, Priority: 5, Patterns: []string{"cmake-build-*"}, Signatures: []string{"CMakeLists.txt", "Makefile", "configure", "configure.ac"}},
			{Name: "flutter", Enabled: true, Priority: 5, Patterns: []string{"build", ".dart_tool"}, Signatures: []string{"pubspec.yaml", "pubspec.lock"}},
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

// ----- Main -----

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

	cfg, err := loadConfig(*flagConfig)
	if err != nil {
		fmt.Println("config error:", err)
		fmt.Println("Tip: run with --init to create a starter config")
		os.Exit(1)
	}

	// Determine scan path
	scanPath := cfg.Options.DefaultScanPath
	if *flagScan != "" {
		scanPath = *flagScan
	}
	scanPath = expand(scanPath)

	// Determine max depth
	maxDepth := cfg.Options.MaxDepth
	if *flagDepth > 0 {
		maxDepth = *flagDepth
	}

	// Filter languages
	selectedLangs := map[string]bool{}
	if *flagLangs != "" {
		for _, l := range strings.Split(*flagLangs, ",") {
			if s := strings.TrimSpace(strings.ToLower(l)); s != "" {
				selectedLangs[s] = true
			}
		}
	}

	var languages []Language
	var allPatterns []string
	patternToLang := make(map[string]string)
	langSignatures := make(map[string][]string)
	langPriorities := make(map[string]int)
	langToPatterns := make(map[string][]string)
	for _, lang := range cfg.Languages {
		if !lang.Enabled {
			continue
		}
		if len(selectedLangs) > 0 && !selectedLangs[strings.ToLower(lang.Name)] {
			continue
		}
		languages = append(languages, lang)
		for _, pattern := range lang.Patterns {
			allPatterns = append(allPatterns, pattern)
			patternToLang[pattern] = lang.Name
		}
		// Build language signature, priority, and pattern maps
		if len(lang.Signatures) > 0 {
			langSignatures[lang.Name] = lang.Signatures
		}
		// Use priority from config, default to 5 if not set (0 means not set in YAML)
		priority := lang.Priority
		if priority == 0 {
			priority = 5
		}
		langPriorities[lang.Name] = priority
		langToPatterns[lang.Name] = lang.Patterns
	}

	if len(languages) == 0 {
		fmt.Println("No languages selected.")
		os.Exit(0)
	}

	rep := Report{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		DryRun:   !*flagClean,
		When:     time.Now(),
		ScanPath: scanPath,
		MaxDepth: maxDepth,
		Findings: []Finding{},
		Warnings: []string{},
	}
	if h, _ := os.Hostname(); h != "" {
		rep.Hostname = h
	}

	// Scan for cache directories
	fmt.Printf("Scanning %s (max depth: %d)...\n", scanPath, maxDepth)
	if cfg.Options.DetectLanguage {
		fmt.Printf("Language detection enabled - scanning with language-specific patterns\n")
	}
	findings := scanDirectory(scanPath, maxDepth, allPatterns, patternToLang, cfg.Options.DetectLanguage, langSignatures, langPriorities, langToPatterns)
	rep.Findings = findings

	var total int64
	for _, f := range findings {
		total += f.SizeBytes
	}
	rep.Total = total

	// Output results
	if *flagJSON {
		b, err := json.MarshalIndent(rep, "", "  ")
		if err != nil {
			fmt.Println("json error:", err)
			os.Exit(1)
		}
		fmt.Println(string(b))
		return
	}

	fmt.Printf("Config: %s\n", *flagConfig)
	fmt.Printf("Scan: %s\n", rep.When.Format(time.RFC3339))
	fmt.Printf("Dry-run: %v\n\n", rep.DryRun)

	if len(findings) == 0 {
		fmt.Println("No cache directories found.")
		return
	}

	// Display results
	displayDetailed(findings, total)

	// Cleanup if requested
	if *flagClean {
		// Filter findings to only include those with a cache pattern (non-empty Pattern)
		var cacheFindings []Finding
		var cacheTotal int64
		for _, f := range findings {
			if isCacheDirectory(f) {
				cacheFindings = append(cacheFindings, f)
				cacheTotal += f.SizeBytes
			}
		}

		if len(cacheFindings) == 0 {
			fmt.Println("\nNo cache directories found to delete.")
			return
		}

		if !*flagYes {
			fmt.Printf("\nWARNING: This will delete %d cache directories:\n", len(cacheFindings))
			// Sort findings by size (largest first) for display
			sortedFindings := make([]Finding, len(cacheFindings))
			copy(sortedFindings, cacheFindings)
			sort.Slice(sortedFindings, func(i, j int) bool {
				return sortedFindings[i].SizeBytes > sortedFindings[j].SizeBytes
			})
			for _, f := range sortedFindings {
				fmt.Printf("  - %s (%s)\n", f.Path, human(f.SizeBytes))
			}
			fmt.Printf("\nContinue? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("input error:", err)
				return
			}
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Cancelled.")
				return
			}
		}

		fmt.Println("\nDeleting cache directories...")
		beforeTotal := cacheTotal
		var deletedCount int
		var errors []string

		for _, f := range cacheFindings {
			if err := os.RemoveAll(f.Path); err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", f.Path, err))
				continue
			}
			deletedCount++
		}

		// Re-scan to verify
		fmt.Println("Re-scanning after cleanup...")
		afterFindings := scanDirectory(scanPath, maxDepth, allPatterns, patternToLang, cfg.Options.DetectLanguage, langSignatures, langPriorities, langToPatterns)
		var afterTotal int64
		for _, f := range afterFindings {
			if isCacheDirectory(f) {
				afterTotal += f.SizeBytes
			}
		}

		freed := beforeTotal - afterTotal
		fmt.Printf("\nDeleted %d directories", deletedCount)
		if freed > 0 {
			fmt.Printf(", freed %s", human(freed))
		}
		fmt.Println()

		if len(errors) > 0 {
			fmt.Println("\nErrors:")
			for _, e := range errors {
				fmt.Printf("  - %s\n", e)
			}
		}
	}

	if len(rep.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, w := range rep.Warnings {
			fmt.Println(" -", w)
		}
	}
}

// detectLanguage checks a directory for language signature files and returns the detected language name
// Returns empty string if no language is detected
// Languages are checked in priority order (more specific frameworks first)
// Multiple languages can have the same priority - they will be checked in map iteration order
func detectLanguage(dirPath string, langSignatures map[string][]string, langPriorities map[string]int) string {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return ""
	}

	// Build list of languages and sort by priority
	type langEntry struct {
		name       string
		signatures []string
		priority   int
	}
	var langs []langEntry
	for lang, signatures := range langSignatures {
		priority, ok := langPriorities[lang]
		if !ok || priority == 0 {
			priority = 5 // Default priority for languages not in the map or not set
		}
		langs = append(langs, langEntry{
			name:       lang,
			signatures: signatures,
			priority:   priority,
		})
	}

	// Sort by priority (lower number = higher priority = checked first)
	// Languages with the same priority will maintain their relative order
	sort.Slice(langs, func(i, j int) bool {
		return langs[i].priority < langs[j].priority
	})

	// Check for signature files in priority order
	for _, langEntry := range langs {
		for _, sig := range langEntry.signatures {
			// Handle wildcard patterns (e.g., "*.csproj")
			if strings.HasPrefix(sig, "*.") {
				ext := strings.TrimPrefix(sig, "*")
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), strings.ToLower(ext)) {
						return langEntry.name
					}
				}
			} else {
				// Exact file match
				for _, entry := range entries {
					if !entry.IsDir() && entry.Name() == sig {
						return langEntry.name
					}
				}
			}
		}
	}

	return ""
}

// scanDirectory walks through the directory tree up to maxDepth and matches directory names against patterns
func scanDirectory(root string, maxDepth int, patterns []string, patternToLang map[string]string, detectLang bool, langSignatures map[string][]string, langPriorities map[string]int, langToPatterns map[string][]string) []Finding {
	var findings []Finding

	// Directories that should not have language detection performed
	// These directories will still be listed but with null/empty language
	excludedFromLanguageDetection := []string{
		".git",
	}

	// Normalize root path for consistent separator counting
	root = filepath.Clean(root)
	rootAbs, err := filepath.Abs(root)
	if err == nil {
		root = rootAbs
	}

	// Cache for language detection results to avoid repeated directory reads
	langCache := make(map[string]string)
	// Track depth 0 directories without languages to report at max depth
	depth0NoLang := make(map[string]bool)
	// Track all depth 0 directories that were scanned (for final reporting)
	depth0Scanned := make(map[string]bool)
	// Map to track findings by path for O(1) lookups (only for findings with non-empty Pattern)
	findingsByPath := make(map[string]bool)
	// Track excluded directories (will be reported with empty language)
	excludedDirs := make(map[string]bool)

	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		if !d.IsDir() {
			return nil
		}

		// Normalize and get absolute path for comparison
		cleanPath := filepath.Clean(path)
		pathAbs, err := filepath.Abs(cleanPath)
		if err == nil {
			cleanPath = pathAbs
		}

		// Skip the root directory itself
		if cleanPath == root {
			return nil
		}

		// Calculate current depth relative to root
		// Count separators in the relative path from root
		relPath, err := filepath.Rel(root, cleanPath)
		if err != nil {
			// Can't calculate relative path, skip
			return nil
		}

		// Count separators in relative path to get depth
		// Empty relPath means root (shouldn't happen due to check above)
		// "proj1" = depth 0 (but we treat as depth 1 for the first level)
		// "proj1/node_modules" = depth 1
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth > maxDepth {
			return filepath.SkipDir
		}

		// Determine which patterns to check for this directory
		patternsToCheck := patterns
		var detectedLang string
		var projectRoot string

		// First, check if this directory matches a cache pattern
		// Cache directories should not be treated as project roots
		dirName := filepath.Base(cleanPath)
		matchesCachePattern := false
		for _, pattern := range patterns {
			if matchPattern(dirName, pattern) {
				matchesCachePattern = true
				break
			}
		}

		// Check if this directory should be excluded from language detection
		isExcluded := false
		for _, excluded := range excludedFromLanguageDetection {
			if dirName == excluded {
				isExcluded = true
				break
			}
		}

		if detectLang {
			// Detect language at project root level (depth 0 or 1)
			// For depth 0, this is a direct child of root (first project level)
			// For depth > 0, we're deeper in the tree - check parent directories for language
			if depth == 0 && !matchesCachePattern {
				// This is a project root directory - detect language here
				// But skip if it matches a cache pattern (cache dirs are not project roots)
				projectRoot = cleanPath
			} else if depth == 0 && matchesCachePattern {
				// This is a cache directory at depth 0 - use scan root as project root
				projectRoot = root
			} else {
				// We're deeper in the tree - walk up to find the project root
				// Find the project root (first directory at depth 0 from root)
				parts := strings.Split(relPath, string(filepath.Separator))
				if len(parts) > 0 {
					projectRootAbs, err := filepath.Abs(filepath.Join(root, parts[0]))
					if err == nil {
						projectRoot = projectRootAbs
					} else {
						projectRoot = filepath.Join(root, parts[0])
					}
				}
			}

			// Track depth 0 directories
			if depth == 0 {
				depth0Scanned[projectRoot] = true
			}

			// If this directory is excluded, skip language detection and mark it
			if isExcluded {
				// If this is the project root itself (depth 0), mark the project root as excluded
				if depth == 0 {
					excludedDirs[projectRoot] = true
					langCache[projectRoot] = "" // Explicitly set to empty, not "no language found"
				}
				detectedLang = ""
				// Skip all language detection for this directory at any depth
			}

			// Check cache first for project root
			if cachedLang, ok := langCache[projectRoot]; ok {
				detectedLang = cachedLang
				// Remove from tracking if language was found
				if detectedLang != "" {
					delete(depth0NoLang, projectRoot)
				}
			} else if projectRoot != "" && !excludedDirs[projectRoot] {
				// Detect language at project root and cache result (skip if excluded)
				detectedLang = detectLanguage(projectRoot, langSignatures, langPriorities)
				if projectRoot != "" {
					langCache[projectRoot] = detectedLang
					// Track depth 0 directories without languages
					if depth == 0 && detectedLang == "" {
						depth0NoLang[projectRoot] = true
					}
				}
			}

			// If no language found at project root and we haven't reached max depth,
			// try detecting language at current directory level (might be a nested project)
			if detectedLang == "" && depth < maxDepth && !excludedDirs[projectRoot] {
				// Try detecting language at current directory (skip if excluded)
				currentLang := detectLanguage(cleanPath, langSignatures, langPriorities)
				if currentLang != "" {
					detectedLang = currentLang
					// Cache this as a project root for this subtree
					langCache[cleanPath] = detectedLang
				}
				// Continue walking deeper to find language or reach max depth
				// Don't return early - we still need to scan for patterns at this level
			}

			// Try detecting language at current level if we're at max depth and haven't found one yet
			if depth == maxDepth && detectedLang == "" && !excludedDirs[projectRoot] {
				// Try one more time to detect language at current level (skip if excluded)
				currentLang := detectLanguage(cleanPath, langSignatures, langPriorities)
				if currentLang == "" {
					// Only report "no language found" for directories at depth 0 (project roots)
					// Don't report for subdirectories like .git, docs, etc. that are not project roots
					if depth == 0 {
						f := Finding{
							Path:        path,
							ProjectRoot: cleanPath, // Same as Path since this is the project root
							Language:    getLanguageForExclusion("", excludedDirs[cleanPath]),
							Pattern:     "",
							SizeBytes:   0,
							Items:       0,
							ModMax:      time.Time{},
						}
						findings = append(findings, f)
						// Remove from tracking since we've reported it
						delete(depth0NoLang, cleanPath)
					}
					// At max depth with no language, still scan for patterns using all patterns
					// (patternsToCheck already contains all patterns)
					// Don't report "no language found" here - pattern matching happens below
					// and fallback reporting happens at the end
				} else {
					detectedLang = currentLang
					langCache[cleanPath] = currentLang
				}
			}

			// When we reach max depth, check if we need to report any depth 0 directories without languages
			if depth == maxDepth && projectRoot != "" {
				if _, needsReport := depth0NoLang[projectRoot]; needsReport {
					// Check if project root still doesn't have a language
					if cachedLang, ok := langCache[projectRoot]; !ok || cachedLang == "" {
						f := Finding{
							Path:        projectRoot,
							ProjectRoot: projectRoot, // Same as Path since this is the project root
							Language:    getLanguageForExclusion("", excludedDirs[projectRoot]),
							Pattern:     "",
							SizeBytes:   0,
							Items:       0,
							ModMax:      time.Time{},
						}
						findings = append(findings, f)
						// Remove from tracking since we've reported it
						delete(depth0NoLang, projectRoot)
					}
				}
			}

			// If language detected, use only patterns for that language
			if detectedLang != "" {
				if langPatterns, ok := langToPatterns[detectedLang]; ok {
					patternsToCheck = langPatterns
				}
			}
		}

		// Check if directory name matches any pattern BEFORE any fallback reporting
		// This ensures cache directories are detected even if they're at depth 0
		for _, pattern := range patternsToCheck {
			if matchPattern(dirName, pattern) {
				// Found a match - calculate size
				f, err := inspectPath(path)
				if err != nil {
					f.Err = err.Error()
				}
				f.Pattern = pattern
				// Use detected language when available; patternToLang can be wrong when
				// multiple languages share a pattern (e.g. node_modules in node, nextjs, vue)
				if detectLang && detectedLang != "" {
					f.Language = detectedLang
				} else {
					f.Language = patternToLang[pattern]
				}
				// Set project root to where language was detected, or parent if no language detection
				if detectLang && projectRoot != "" {
					f.ProjectRoot = projectRoot
				} else {
					// If no language detection, use parent directory as project root
					f.ProjectRoot = filepath.Dir(cleanPath)
				}
				findings = append(findings, f)
				// Track this path in the map for O(1) lookups
				findingsByPath[cleanPath] = true

				// Skip subdirectories of matched directories
				return filepath.SkipDir
			}
		}

		// Only report "no language found" at max depth if we didn't match a pattern
		// and this is a depth 0 directory (project root)
		if detectLang && depth == 0 && detectedLang == "" && !matchesCachePattern {
			// Check if this directory already has a finding (as a cache directory) using O(1) map lookup
			hasFinding := findingsByPath[cleanPath]
			// Only report if no cache directory finding exists
			if !hasFinding {
				// Check if we need to report this depth 0 directory
				if _, needsReport := depth0NoLang[projectRoot]; needsReport {
					f := Finding{
						Path:        projectRoot,
						ProjectRoot: projectRoot,
						Language:    "no language found",
						Pattern:     "",
						SizeBytes:   0,
						Items:       0,
						ModMax:      time.Time{},
					}
					findings = append(findings, f)
					delete(depth0NoLang, projectRoot)
				}
			}
		}

		return nil
	}); err != nil {
		// Log walk error but continue processing findings
		fmt.Fprintf(os.Stderr, "warning: directory walk error: %v\n", err)
	}

	// After scanning, report all depth 0 directories that were scanned but don't have cache directory findings
	// This ensures directories with detected languages are always reported, even if they don't have cache directories
	// Track which depth 0 directories have findings (cache directories found)
	depth0HasFindings := make(map[string]bool)
	for _, f := range findings {
		// Check if this finding is for a depth 0 directory or a subdirectory of one
		for depth0Root := range depth0Scanned {
			if f.Path == depth0Root || strings.HasPrefix(f.Path, depth0Root+string(filepath.Separator)) {
				depth0HasFindings[depth0Root] = true
				break
			}
		}
	}

	// Report all depth 0 directories that don't have cache directory findings
	// This includes directories with detected languages (even if no cache directories found)
	for projectRoot := range depth0Scanned {
		if !depth0HasFindings[projectRoot] {
			// Get the detected language for this directory
			// Always report the language if found, even if there are no cache directories
			detectedLang := langCache[projectRoot]
			f := Finding{
				Path:        projectRoot,
				ProjectRoot: projectRoot, // Same as Path since this is the project root
				Language:    getLanguageForExclusion(detectedLang, excludedDirs[projectRoot]),
				Pattern:     "",
				SizeBytes:   0,
				Items:       0,
				ModMax:      time.Time{},
			}
			findings = append(findings, f)
		}
	}

	return findings
}

// matchPattern checks if a directory name matches a pattern
// Supports exact match and simple wildcard patterns
func matchPattern(name, pattern string) bool {
	// Exact match
	if name == pattern {
		return true
	}

	// Simple wildcard support (e.g., "cmake-build-*")
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(name, prefix)
	}

	return false
}

func displayDetailed(findings []Finding, total int64) {
	// Group findings by project path, then by cache type
	type cacheTypeSummary struct {
		SizeBytes int64
		Items     int
		Language  string
		Patterns  []string // Track all patterns of this type
	}

	// First pass: identify which projects have actual cache directories
	projectsWithCache := make(map[string]bool)
	for _, f := range findings {
		if f.Err != "" {
			continue
		}
		if isCacheDirectory(f) {
			// Use ProjectRoot if available, otherwise fall back to Path
			projectPath := f.ProjectRoot
			if projectPath == "" {
				projectPath = f.Path
			}
			projectsWithCache[projectPath] = true
		}
	}

	// Map: ProjectRoot -> CacheType -> Summary
	projectGroups := make(map[string]map[string]*cacheTypeSummary)

	for _, f := range findings {
		if f.Err != "" {
			continue
		}
		// Use ProjectRoot if available, otherwise fall back to Path
		projectPath := f.ProjectRoot
		if projectPath == "" {
			projectPath = f.Path
		}

		// Skip "(no cache directories)" entries if this project has actual cache directories
		if f.Pattern == "" && projectsWithCache[projectPath] {
			continue
		}

		// Use Pattern as cache type key, or "no cache" if empty
		cacheType := f.Pattern
		if cacheType == "" {
			cacheType = "(no cache directories)"
		}

		if projectGroups[projectPath] == nil {
			projectGroups[projectPath] = make(map[string]*cacheTypeSummary)
		}

		if projectGroups[projectPath][cacheType] == nil {
			projectGroups[projectPath][cacheType] = &cacheTypeSummary{
				Language: f.Language,
				Patterns: []string{},
			}
		}

		projectGroups[projectPath][cacheType].SizeBytes += f.SizeBytes
		projectGroups[projectPath][cacheType].Items += f.Items
		if isCacheDirectory(f) && !contains(projectGroups[projectPath][cacheType].Patterns, f.Pattern) {
			projectGroups[projectPath][cacheType].Patterns = append(projectGroups[projectPath][cacheType].Patterns, f.Pattern)
		}
	}

	// Calculate totals for each project and prepare for sorting
	type projectEntry struct {
		path       string
		totalSize  int64
		totalItems int
		cacheTypes map[string]*cacheTypeSummary
		language   string
	}

	var projectEntries []projectEntry
	for path, cacheTypes := range projectGroups {
		var projectTotalSize int64
		var projectTotalItems int
		var detectedLanguage string

		// Calculate totals and find language
		for _, summary := range cacheTypes {
			projectTotalSize += summary.SizeBytes
			projectTotalItems += summary.Items
			if detectedLanguage == "" && summary.Language != "" {
				detectedLanguage = summary.Language
			}
		}

		projectEntries = append(projectEntries, projectEntry{
			path:       path,
			totalSize:  projectTotalSize,
			totalItems: projectTotalItems,
			cacheTypes: cacheTypes,
			language:   detectedLanguage,
		})
	}

	// Sort projects by total size (largest first)
	sort.Slice(projectEntries, func(i, j int) bool {
		return projectEntries[i].totalSize > projectEntries[j].totalSize
	})

	// Display grouped results - one line per project with all cache types listed
	table := tablewriter.NewWriter(os.Stdout)
	table.Header("Project Path", "Cache Types", "Language", "Total Size", "Total Items")

	var totalItems int
	for _, project := range projectEntries {
		cacheTypes := project.cacheTypes

		// Sort cache types by size (largest first) within each project
		type cacheTypeEntry struct {
			name    string
			summary *cacheTypeSummary
		}
		var sortedCacheTypes []cacheTypeEntry
		for cacheType, summary := range cacheTypes {
			sortedCacheTypes = append(sortedCacheTypes, cacheTypeEntry{
				name:    cacheType,
				summary: summary,
			})
		}
		sort.Slice(sortedCacheTypes, func(i, j int) bool {
			return sortedCacheTypes[i].summary.SizeBytes > sortedCacheTypes[j].summary.SizeBytes
		})

		// Build cache types list string
		var cacheTypeList []string
		for _, entry := range sortedCacheTypes {
			cacheTypeList = append(cacheTypeList, entry.name)
		}

		// Join cache types with semicolons for readability
		cacheTypesStr := strings.Join(cacheTypeList, "; ")

		// Add single row for this project
		if err := table.Append(project.path, cacheTypesStr, project.language, human(project.totalSize), fmt.Sprintf("%d", project.totalItems)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to append table row: %v\n", err)
		}

		totalItems += project.totalItems
	}

	table.Footer("TOTAL", "", "", human(total), fmt.Sprintf("%d", totalItems))
	if err := table.Render(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to render table: %v\n", err)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
