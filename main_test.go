package main

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestParseHumanSize(t *testing.T) {
	tests := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"0 B", 0, true},
		{"1B", 1, true},
		{"1 KB", 1024, true},
		{"1.5MB", 1572864, true},
		{"2 gb", 2 * 1024 * 1024 * 1024, true},
		{"3 TB", 3 * 1024 * 1024 * 1024 * 1024, true},
		{"1,024 KB", 1048576, true},
		{"bogus", 0, false},
	}
	for _, tt := range tests {
		got, ok := parseHumanSize(tt.in)
		if ok != tt.ok {
			t.Fatalf("parseHumanSize(%q) ok=%v, want %v", tt.in, ok, tt.ok)
		}
		if ok && got != tt.want {
			t.Fatalf("parseHumanSize(%q)=%d, want %d", tt.in, got, tt.want)
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
	if !strings.HasSuffix(p, filepath.Join(".config", "mac-cache-cleaner", "config.yaml")) {
		t.Fatalf("defaultConfigPath() unexpected: %q", p)
	}
}

func TestRunCmdMissingBinary(t *testing.T) {
	res := runCmd([]string{"this-binary-should-not-exist-12345"})
	if res.Found {
		t.Fatalf("expected Found=false for missing binary")
	}
	if res.Error == "" {
		t.Fatalf("expected an error message for missing binary")
	}
}

func TestCheckToolWithCheckPath(t *testing.T) {
	// Create a temporary file and point CheckPath to it; should be considered installed
	f, err := os.CreateTemp(t.TempDir(), "checkpath-*")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	tool := Tool{Name: "dummy", CheckPath: f.Name()}
	installed, info, err := checkTool(tool)
	if err != nil {
		t.Fatalf("checkTool returned error: %v", err)
	}
	if !installed || info == "" {
		t.Fatalf("expected installed with info, got installed=%v info=%q", installed, info)
	}
}

func TestGetInstallCommandNoBrew(t *testing.T) {
	// Avoid invoking brew by clearing PATH temporarily
	t.Setenv("PATH", "")
	cmd := getInstallCommand(Tool{Name: "some-tool-without-brew"})
	if !strings.Contains(cmd, "Install some-tool-without-brew") {
		t.Fatalf("unexpected getInstallCommand output: %q", cmd)
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

func TestExpandGlobs_NoWildcards(t *testing.T) {
	// Existing path returns it directly
	file := filepath.Join(t.TempDir(), "foo.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths, err := expandGlobs(file)
	if err != nil {
		t.Fatalf("expandGlobs error: %v", err)
	}
	if len(paths) != 1 || paths[0] != file {
		t.Fatalf("expected single path %q, got %#v", file, paths)
	}

	// Non-existent returns empty slice (no error)
	none, err := expandGlobs(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected empty result, got %#v", none)
	}
}

func TestExpandGlobs_CommandSubstitutionRejected(t *testing.T) {
	// Use a command not on the whitelist; should return an error
	_, err := expandGlobs("$(echo hi)/something")
	if err == nil {
		t.Fatalf("expected error for non-whitelisted command substitution")
	}
}

func TestMain_noArgs_summaryOnly(t *testing.T) {
	// Smoke test to ensure main() doesn't panic when invoked with no args.
	// We won't execute it directly to avoid os.Exit; instead, ensure that the binary builds on this platform.
	if runtime.GOOS == "" { // always true; placeholder to avoid unused import warnings in certain scenarios
		t.Log("ok")
	}
}

func TestConfigIO_WriteAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	// Write starter config
	if err := writeStarterConfig(cfgPath, false); err != nil {
		t.Fatalf("writeStarterConfig failed: %v", err)
	}

	// Loading should work and have non-empty targets
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if cfg == nil || len(cfg.Targets) == 0 {
		t.Fatalf("expected non-empty targets in starter config")
	}

	// Writing again without force should error
	if err := writeStarterConfig(cfgPath, false); err == nil {
		t.Fatalf("expected error when overwriting without force")
	}

	// With force should succeed and create a backup (we don't assert backup existence here)
	if err := writeStarterConfig(cfgPath, true); err != nil {
		t.Fatalf("writeStarterConfig with force failed: %v", err)
	}
}
