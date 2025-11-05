package main

import (
	"os"
	"path/filepath"
	"testing"
)

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
