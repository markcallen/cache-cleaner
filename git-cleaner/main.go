package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
)

// ----- Version info -----
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// ----- CLI flags -----
var (
	flagScan  = flag.String("scan", "", "Directory to scan for .git directories")
	flagClean = flag.Bool("clean", false, "Run git gc in each repository and show disk savings")
)

// ----- Finding types -----

type Finding struct {
	Path      string `json:"path"`
	RepoPath  string `json:"repo_path"` // Parent directory containing .git
	SizeBytes int64  `json:"size_bytes"`
	Items     int    `json:"items"`
}

// ----- Utilities -----

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
	errWalk := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
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

// scanDirectory walks through the directory tree and finds all .git directories
func scanDirectory(root string) []Finding {
	var findings []Finding

	root = filepath.Clean(root)
	rootAbs, err := filepath.Abs(root)
	if err == nil {
		root = rootAbs
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		if !d.IsDir() {
			return nil
		}

		// Check if this directory is named .git
		if d.Name() == ".git" {
			// Get the repository path (parent of .git)
			repoPath := filepath.Dir(path)

			// Inspect the .git directory
			f, err := inspectPath(path)
			if err != nil {
				return nil
			}
			f.RepoPath = repoPath
			findings = append(findings, f)

			// Skip subdirectories of .git
			return filepath.SkipDir
		}

		return nil
	})
	if err != nil {
		// Log error but continue - return what we found so far
		fmt.Fprintf(os.Stderr, "Warning: error walking directory: %v\n", err)
	}

	return findings
}

// runGitGC runs git gc in the specified repository directory
func runGitGC(repoPath string) error {
	cmd := exec.Command("git", "gc")
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// displayResults displays findings in a table
func displayResults(findings []Finding, total int64) {
	if len(findings) == 0 {
		fmt.Println("No .git directories found.")
		return
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header("Repository Path", ".git Size", "Items")

	// Sort by size (largest first)
	sortedFindings := make([]Finding, len(findings))
	copy(sortedFindings, findings)
	sort.Slice(sortedFindings, func(i, j int) bool {
		return sortedFindings[i].SizeBytes > sortedFindings[j].SizeBytes
	})

	var totalItems int
	for _, f := range sortedFindings {
		if err := table.Append(f.RepoPath, human(f.SizeBytes), fmt.Sprintf("%d", f.Items)); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: error appending to table: %v\n", err)
		}
		totalItems += f.Items
	}

	table.Footer("TOTAL", human(total), fmt.Sprintf("%d", totalItems))
	if err := table.Render(); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering table: %v\n", err)
	}
}

// expandScanPath expands ~ and env vars, resolves to absolute path, and verifies it exists.
// Returns the expanded path or an error.
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

func main() {
	if checkVersionFlag() {
		fmt.Printf("version %s, commit %s, built at %s\n", version, commit, date)
		return
	}

	flag.Parse()

	scanPath, err := expandScanPath(*flagScan)
	if err != nil {
		if err.Error() == "scan path is required" {
			fmt.Println("Error: --scan flag is required")
			fmt.Println("Usage: git-cleaner --scan <directory> [--clean]")
		} else {
			fmt.Printf("Error: %v\n", err)
		}
		os.Exit(1)
	}

	fmt.Printf("Scanning %s for .git directories...\n", scanPath)

	// Initial scan
	findings := scanDirectory(scanPath)

	var totalBefore int64
	for _, f := range findings {
		totalBefore += f.SizeBytes
	}

	if len(findings) == 0 {
		fmt.Println("No .git directories found.")
		return
	}

	// Display initial results
	fmt.Printf("\nFound %d repositories:\n\n", len(findings))
	displayResults(findings, totalBefore)

	// Clean if requested
	if *flagClean {
		fmt.Printf("\nRunning git gc in %d repositories...\n", len(findings))
		var errors []string
		var cleanedCount int

		for _, f := range findings {
			fmt.Printf("  Cleaning %s...\n", f.RepoPath)
			if err := runGitGC(f.RepoPath); err != nil {
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

		// Rescan after cleanup
		fmt.Println("\nRescanning after cleanup...")
		time.Sleep(100 * time.Millisecond) // Brief pause to ensure file system updates
		afterFindings := scanDirectory(scanPath)

		var totalAfter int64
		for _, f := range afterFindings {
			totalAfter += f.SizeBytes
		}

		diskSavings := totalBefore - totalAfter
		fmt.Printf("\nResults after cleanup:\n\n")
		displayResults(afterFindings, totalAfter)

		fmt.Printf("\nDisk savings: %s (%.2f%%)\n",
			human(diskSavings),
			float64(diskSavings)/float64(totalBefore)*100)
		fmt.Printf("Cleaned %d repositories\n", cleanedCount)
	}
}
