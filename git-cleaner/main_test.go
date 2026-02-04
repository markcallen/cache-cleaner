package main

import (
	"bytes"
	"os"
	"os/exec"
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
		{"other flags", []string{"program", "--scan", "/tmp"}, false},
		{"version with other flags", []string{"program", "--scan", "/tmp", "--version"}, true},
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
		{1024 * 1024 * 1024 * 1024, "1.00 TB"},
	}
	for _, tt := range tests {
		got := human(tt.in)
		if got != tt.want {
			t.Fatalf("human(%d)=%q, want %q", tt.in, got, tt.want)
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
	if f.Path != dir {
		t.Fatalf("expected path %q, got %q", dir, f.Path)
	}
}

func TestInspectPathFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(filePath, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := inspectPath(filePath)
	if err != nil {
		t.Fatalf("inspectPath error: %v", err)
	}
	if f.Items != 1 {
		t.Fatalf("expected 1 item, got %d", f.Items)
	}
	if f.SizeBytes != 11 {
		t.Fatalf("expected size 11, got %d", f.SizeBytes)
	}
	if f.Path != filePath {
		t.Fatalf("expected path %q, got %q", filePath, f.Path)
	}
}

func TestInspectPathNonExistent(t *testing.T) {
	dir := t.TempDir()
	nonExistent := filepath.Join(dir, "nonexistent")
	_, err := inspectPath(nonExistent)
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
}

func TestScanDirectory(t *testing.T) {
	root := t.TempDir()

	// Create test structure:
	// root/
	//   proj1/
	//     .git/  (should find this)
	//   proj2/
	//     .git/  (should find this)
	//   proj3/
	//     subdir/
	//       .git/  (should find this)

	proj1 := filepath.Join(root, "proj1")
	if err := os.MkdirAll(proj1, 0o755); err != nil {
		t.Fatal(err)
	}
	git1 := filepath.Join(proj1, ".git")
	if err := os.MkdirAll(git1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(git1, "config"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj2 := filepath.Join(root, "proj2")
	if err := os.MkdirAll(proj2, 0o755); err != nil {
		t.Fatal(err)
	}
	git2 := filepath.Join(proj2, ".git")
	if err := os.MkdirAll(git2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(git2, "HEAD"), []byte("ref: refs/heads/main"), 0o644); err != nil {
		t.Fatal(err)
	}

	proj3 := filepath.Join(root, "proj3")
	if err := os.MkdirAll(filepath.Join(proj3, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	git3 := filepath.Join(proj3, "subdir", ".git")
	if err := os.MkdirAll(git3, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(git3, "index"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Test scanning
	findings := scanDirectory(root)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(findings))
	}

	// Verify repo paths are correct
	repoPaths := make(map[string]bool)
	for _, f := range findings {
		repoPaths[f.RepoPath] = true
		if f.Path == "" {
			t.Fatal("expected Path to be set")
		}
		if f.RepoPath == "" {
			t.Fatal("expected RepoPath to be set")
		}
	}

	if !repoPaths[proj1] {
		t.Fatalf("expected to find repo path %q", proj1)
	}
	if !repoPaths[proj2] {
		t.Fatalf("expected to find repo path %q", proj2)
	}
	if !repoPaths[filepath.Join(proj3, "subdir")] {
		t.Fatalf("expected to find repo path %q", filepath.Join(proj3, "subdir"))
	}
}

func TestScanDirectoryInspectPathError(t *testing.T) {
	// .git dir that inspectPath might fail on (e.g. permission)
	root := t.TempDir()
	proj := filepath.Join(root, "proj")
	gitDir := filepath.Join(proj, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Make objects dir unreadable to cause walk error
	objectsDir := filepath.Join(gitDir, "objects")
	if err := os.MkdirAll(objectsDir, 0o000); err != nil {
		t.Skip("cannot create restricted dir")
	}
	defer func() { _ = os.Chmod(objectsDir, 0o755) }()

	findings := scanDirectory(root)
	// Should still find the repo (inspectPath may return partial or error)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestScanDirectoryNoGitDirs(t *testing.T) {
	root := t.TempDir()

	// Create structure without .git directories
	if err := os.MkdirAll(filepath.Join(root, "proj1", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "proj1", "src", "file.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}

	findings := scanDirectory(root)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func TestScanDirectoryNestedGit(t *testing.T) {
	root := t.TempDir()

	// Create structure with nested .git directories
	// root/
	//   proj1/
	//     .git/
	//       objects/  (should be skipped, not scanned separately)
	proj1 := filepath.Join(root, "proj1")
	if err := os.MkdirAll(proj1, 0o755); err != nil {
		t.Fatal(err)
	}
	git1 := filepath.Join(proj1, ".git")
	if err := os.MkdirAll(filepath.Join(git1, "objects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(git1, "config"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(git1, "objects", "pack"), []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}

	findings := scanDirectory(root)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding (nested .git should be skipped), got %d", len(findings))
	}
	if findings[0].RepoPath != proj1 {
		t.Fatalf("expected repo path %q, got %q", proj1, findings[0].RepoPath)
	}
}

func TestRunGitGC(t *testing.T) {
	// Create a real git repo and run gc
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Skip("git not available or init failed:", err)
	}
	// Add a file so there's something to gc
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", dir, "add", "README").Run(); err != nil {
		t.Skip("git add failed:", err)
	}
	if err := exec.Command("git", "-C", dir, "commit", "-m", "init").Run(); err != nil {
		t.Skip("git commit failed:", err)
	}

	if err := runGitGC(dir); err != nil {
		t.Fatalf("runGitGC failed: %v", err)
	}
}

func TestDisplayResults(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	findings := []Finding{
		{Path: "/repo1/.git", RepoPath: "/repo1", SizeBytes: 1024, Items: 10},
		{Path: "/repo2/.git", RepoPath: "/repo2", SizeBytes: 2048, Items: 5},
	}
	displayResults(findings, 3072)

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()
	if !strings.Contains(out, "repo1") || !strings.Contains(out, "repo2") {
		t.Fatalf("expected repo paths in output, got: %s", out)
	}
	if !strings.Contains(out, "1.00 KB") || !strings.Contains(out, "2.00 KB") {
		t.Fatalf("expected size formatting in output, got: %s", out)
	}
}

func TestDisplayResultsEmpty(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	displayResults([]Finding{}, 0)
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "No .git directories found") {
		t.Fatalf("expected empty message, got: %s", buf.String())
	}
}

func TestHumanTB(t *testing.T) {
	got := human(1024 * 1024 * 1024 * 1024)
	if !strings.Contains(got, "TB") {
		t.Fatalf("human(1TB) = %q, expected TB unit", got)
	}
	// Test very large to hit all units in the loop
	got2 := human(1024 * 1024 * 1024 * 1024 * 1024)
	if !strings.Contains(got2, "TB") {
		t.Fatalf("human(1PB) = %q, expected TB unit", got2)
	}
}

func TestExpandScanPath(t *testing.T) {
	dir := t.TempDir()

	// Empty path
	_, err := expandScanPath("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
	if !strings.Contains(err.Error(), "scan path is required") {
		t.Fatalf("expected 'scan path is required' error, got %v", err)
	}

	// Non-existent path
	_, err = expandScanPath(filepath.Join(dir, "nonexistent"))
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
	if !strings.Contains(err.Error(), "scan path does not exist") {
		t.Fatalf("expected 'scan path does not exist' error, got %v", err)
	}

	// Valid absolute path
	got, err := expandScanPath(dir)
	if err != nil {
		t.Fatalf("expandScanPath failed: %v", err)
	}
	abs, _ := filepath.Abs(dir)
	if got != abs {
		t.Fatalf("expandScanPath = %q, want %q", got, abs)
	}

	// Expand ~ (if we can)
	home, _ := os.UserHomeDir()
	if home != "" {
		subDir := filepath.Join(home, "tmp")
		if err := os.MkdirAll(subDir, 0o755); err == nil {
			got, err = expandScanPath("~/tmp")
			if err != nil {
				t.Fatalf("expandScanPath failed: %v", err)
			}
			if !strings.HasSuffix(got, "tmp") {
				t.Fatalf("expandScanPath(~/tmp) = %q", got)
			}
			_ = os.RemoveAll(subDir)
		}
		// Expand "~" only (home dir)
		got, err = expandScanPath("~")
		if err != nil {
			t.Fatalf("expandScanPath(~) failed: %v", err)
		}
		homeAbs, _ := filepath.Abs(home)
		if got != homeAbs {
			t.Fatalf("expandScanPath(~) = %q, want %q", got, homeAbs)
		}
	}

	// Env var expansion
	_ = os.Setenv("TEST_SCAN_DIR", dir)
	defer func() { _ = os.Unsetenv("TEST_SCAN_DIR") }()
	got, err = expandScanPath("$TEST_SCAN_DIR")
	if err != nil {
		t.Fatalf("expandScanPath failed: %v", err)
	}
	if got != abs {
		t.Fatalf("expandScanPath($TEST_SCAN_DIR) = %q, want %q", got, abs)
	}
}
