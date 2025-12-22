package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckVersionFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"no args", []string{"program"}, false},
		{"version flag", []string{"program", "--version"}, true},
		{"version flag short", []string{"program", "-version"}, true},
		{"other flags", []string{"program", "--config", "test.yaml"}, false},
		{"version with other flags", []string{"program", "--config", "test.yaml", "--version"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original os.Args
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()

			// Set test args
			os.Args = tt.args

			got := checkVersionFlag()
			if got != tt.want {
				t.Errorf("checkVersionFlag() with args %v = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestHuman(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1.00 KB"},
		{1024*1024 + 512*1024, "1.50 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
	}
	for _, tt := range tests {
		got := human(tt.in)
		if got != tt.want {
			t.Fatalf("human(%d)=%q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExpandHome(t *testing.T) {
	h, _ := os.UserHomeDir()
	got := expand("~")
	if got != h {
		t.Fatalf("expand(~)=%q, want %q", got, h)
	}
	// With env var expansion
	_ = os.Setenv("FOO_TEST_ENV", "xyz")
	got2 := expand("$FOO_TEST_ENV")
	if got2 != "xyz" {
		t.Fatalf("expand($FOO_TEST_ENV)=%q, want %q", got2, "xyz")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	p := defaultConfigPath()
	expectedSuffix := filepath.Join(".config", "dev-cache", "config.yaml")
	if !strings.HasSuffix(p, expectedSuffix) {
		t.Fatalf("defaultConfigPath() unexpected: %q", p)
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		want    bool
	}{
		{"node_modules", "node_modules", true},
		{"node_modules", ".venv", false},
		{"cmake-build-debug", "cmake-build-*", true},
		{"cmake-build-release", "cmake-build-*", true},
		{"cmake-build", "cmake-build-*", false}, // Must have something after prefix
		{"build", "build", true},
		{"build", "build-*", false},
	}
	for _, tt := range tests {
		got := matchPattern(tt.name, tt.pattern)
		if got != tt.want {
			t.Fatalf("matchPattern(%q, %q)=%v, want %v", tt.name, tt.pattern, got, tt.want)
		}
	}
}

func TestInspectPath(t *testing.T) {
	dir := t.TempDir()
	// Create a couple of files
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.bin"), make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := inspectPath(dir)
	if err != nil {
		t.Fatalf("inspectPath error: %v", err)
	}
	if f.Items != 2 {
		t.Fatalf("expected 2 items, got %d", f.Items)
	}
	if f.SizeBytes < 2053 { // 5 + 2048
		t.Fatalf("expected size >= 2053, got %d", f.SizeBytes)
	}
}

func TestScanDirectory(t *testing.T) {
	root := t.TempDir()

	// Create test structure:
	// root/
	//   proj1/
	//     node_modules/  (should find this)
	//   proj2/
	//     .venv/         (should find this)
	//   proj3/
	//     subdir/
	//       node_modules/  (depth 2, should find if maxDepth >= 2)

	proj1 := filepath.Join(root, "proj1")
	if err := os.MkdirAll(proj1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(proj1, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj1, "node_modules", "file.txt"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj2 := filepath.Join(root, "proj2")
	if err := os.MkdirAll(proj2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(proj2, ".venv"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj2, ".venv", "file.txt"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj3 := filepath.Join(root, "proj3")
	if err := os.MkdirAll(filepath.Join(proj3, "subdir", "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj3, "subdir", "node_modules", "file.txt"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	patterns := []string{"node_modules", ".venv"}
	patternToLang := map[string]string{
		"node_modules": "node",
		".venv":        "python",
	}

	// Test with maxDepth 1
	findings := scanDirectory(root, 1, patterns, patternToLang, false, nil, nil, nil)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings with maxDepth=1, got %d", len(findings))
	}

	// Test with maxDepth 2
	findings2 := scanDirectory(root, 2, patterns, patternToLang, false, nil, nil, nil)
	if len(findings2) != 3 {
		t.Fatalf("expected 3 findings with maxDepth=2, got %d", len(findings2))
	}

	// Verify languages are set correctly
	for _, f := range findings {
		if f.Pattern == "node_modules" && f.Language != "node" {
			t.Fatalf("expected language 'node' for node_modules, got %q", f.Language)
		}
		if f.Pattern == ".venv" && f.Language != "python" {
			t.Fatalf("expected language 'python' for .venv, got %q", f.Language)
		}
	}
}

// TestScanDirectoryMapLookup tests the map-based lookup optimization for findingsByPath.
// This ensures that when a cache directory is found at depth 0, it doesn't also get
// a "no language found" report, which would happen if we used a linear search through
// all findings. The map lookup should prevent duplicate reports.
func TestScanDirectoryMapLookup(t *testing.T) {
	root := t.TempDir()

	// Create test structure where a cache directory exists at depth 0 (directly under root)
	// This tests the optimization: the map lookup should prevent duplicate "no language found" reports
	// root/
	//   node_modules/  (depth 0 cache directory - should be found)
	//   proj1/         (depth 0 project - no language, no cache)
	//     file.txt
	//   proj2/         (depth 0 project - has language signature)
	//     package.json
	//     node_modules/  (should be found)

	// Cache directory at depth 0
	nodeModulesRoot := filepath.Join(root, "node_modules")
	if err := os.MkdirAll(nodeModulesRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeModulesRoot, "file.txt"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Project without language signature at depth 0
	proj1 := filepath.Join(root, "proj1")
	if err := os.MkdirAll(proj1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj1, "file.txt"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Project with language signature at depth 0
	proj2 := filepath.Join(root, "proj2")
	if err := os.MkdirAll(proj2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj2, "package.json"), []byte(`{"name": "test"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(proj2, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj2, "node_modules", "file.txt"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	patterns := []string{"node_modules"}
	patternToLang := map[string]string{
		"node_modules": "node",
	}
	langSignatures := map[string][]string{
		"node": {"package.json"},
	}
	langPriorities := map[string]int{
		"node": 10,
	}
	langToPatterns := map[string][]string{
		"node": {"node_modules"},
	}

	// Test with language detection enabled and maxDepth 1
	// This should find:
	// 1. node_modules at root (depth 0) - cache directory finding
	// 2. proj1 - "no language found" report (no signature, no cache)
	// 3. proj2 - node language report (has signature)
	// 4. proj2/node_modules - cache directory finding (depth 1)
	findings := scanDirectory(root, 1, patterns, patternToLang, true, langSignatures, langPriorities, langToPatterns)

	// Count findings by type
	var cacheFindings int
	var noLangFindings int
	nodeModulesRootFound := false
	proj2NodeModulesFound := false

	for _, f := range findings {
		if f.Pattern != "" {
			cacheFindings++
			// Check if we found the root-level node_modules
			if strings.HasSuffix(f.Path, "node_modules") {
				dir := filepath.Dir(f.Path)
				// Normalize paths for comparison
				rootAbs, _ := filepath.Abs(root)
				dirAbs, _ := filepath.Abs(dir)
				if dirAbs == rootAbs {
					nodeModulesRootFound = true
				}
			}
			// Check if we found proj2/node_modules
			if strings.Contains(f.Path, "proj2") && strings.HasSuffix(f.Path, "node_modules") {
				proj2NodeModulesFound = true
			}
		} else if f.Language == "no language found" {
			noLangFindings++
		}
	}

	// Verify the root-level node_modules was found as a cache directory
	if !nodeModulesRootFound {
		t.Fatalf("expected to find node_modules at root level as cache directory. Findings: %+v", findings)
	}

	// Verify proj2/node_modules was found
	if !proj2NodeModulesFound {
		t.Fatalf("expected to find node_modules in proj2 as cache directory. Findings: %+v", findings)
	}

	// Verify we have exactly 2 cache findings (root node_modules and proj2/node_modules)
	if cacheFindings != 2 {
		t.Fatalf("expected 2 cache findings, got %d. Findings: %+v", cacheFindings, findings)
	}

	// Most importantly: verify that node_modules at root doesn't get a duplicate "no language found" report
	// This is what the map optimization prevents - the O(1) map lookup should prevent
	// the linear search through all findings that would cause duplicate reports.
	// The key test: when a cache directory is found at depth 0, it should NOT also get
	// a "no language found" report because the map lookup (findingsByPath) should detect
	// that this path already has a finding with a Pattern.
	rootNodeModulesReports := 0
	var rootNodeModulesFinding Finding
	for _, f := range findings {
		// Normalize paths for comparison
		fPathAbs, _ := filepath.Abs(f.Path)
		nodeModulesRootAbs, _ := filepath.Abs(nodeModulesRoot)
		if fPathAbs == nodeModulesRootAbs {
			rootNodeModulesReports++
			rootNodeModulesFinding = f
			if f.Pattern == "" {
				t.Fatalf("node_modules at root should not get a 'no language found' report - map lookup should prevent this. Finding: %+v", f)
			}
		}
	}
	// Should only have one report for root node_modules (the cache finding)
	if rootNodeModulesReports != 1 {
		t.Fatalf("expected exactly 1 report for root node_modules, got %d. This indicates the map lookup optimization is working correctly", rootNodeModulesReports)
	}
	// Verify the single report is a cache finding (has Pattern)
	if rootNodeModulesFinding.Pattern == "" {
		t.Fatalf("root node_modules finding should have a Pattern set, got: %+v", rootNodeModulesFinding)
	}
}
