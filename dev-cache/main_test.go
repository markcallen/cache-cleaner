package main

import (
	"os"
	"path/filepath"
	"strings"
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
