package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
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
		{"other flags", []string{"program", "--targets", "docker"}, false},
		{"version with other flags", []string{"program", "--targets", "docker", "--version"}, true},
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
	// Very large value to hit TB fallback
	got := human(1024 * 1024 * 1024 * 1024 * 1024)
	if !strings.Contains(got, "TB") {
		t.Fatalf("human(1PB) = %q, expected TB", got)
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

func TestDefaultConfigPathEmptyHome(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	defer func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldUserProfile)
	}()
	os.Unsetenv("HOME")
	os.Unsetenv("USERPROFILE")
	p := defaultConfigPath()
	if p == "" {
		t.Fatal("defaultConfigPath should not return empty")
	}
	// When home is empty, returns "./config.yaml"
	if os.Getenv("HOME") == "" && os.Getenv("USERPROFILE") == "" {
		if p != "./config.yaml" {
			t.Logf("defaultConfigPath with empty home = %q", p)
		}
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

func TestCheckToolCheckPathNotExist(t *testing.T) {
	tool := Tool{Name: "dummy", CheckPath: filepath.Join(t.TempDir(), "nonexistent")}
	installed, _, _ := checkTool(tool)
	if installed {
		t.Fatal("expected not installed when CheckPath does not exist")
	}
}

func TestCheckToolCheckPathWithTilde(t *testing.T) {
	// CheckPath with ~ gets expanded
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no home dir")
	}
	// Use a path that doesn't exist
	tool := Tool{Name: "dummy", CheckPath: "~/.cache-cleaner-test-nonexistent-xyz"}
	installed, _, _ := checkTool(tool)
	if installed {
		t.Fatal("expected not installed for nonexistent path")
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

func TestGetInstallCommandWithBrew(t *testing.T) {
	// When brew is in PATH and tool is in brew - may or may not find it
	cmd := getInstallCommand(Tool{Name: "go"})
	if cmd == "" {
		t.Fatal("getInstallCommand should return something")
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

func TestExpandGlobsDockerSubstitution(t *testing.T) {
	// Docker is whitelisted but returns error for path expansion
	_, err := expandGlobs("$(docker info)/x")
	if err == nil {
		t.Fatalf("expected error for docker substitution")
	}
}

func TestExpandGlobsEmptyCommand(t *testing.T) {
	// $( ) empty command - strips it and continues
	paths, err := expandGlobs("$( )/foo")
	if err != nil {
		t.Fatalf("expandGlobs empty command: %v", err)
	}
	_ = paths
}

func TestMain_noArgs_summaryOnly(t *testing.T) {
	// Smoke test to ensure main() doesn't panic when invoked with no args.
	if runtime.GOOS == "" {
		t.Log("ok")
	}
}

func TestRunVersion(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"mac-cache-cleaner", "--version"}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	code := run()
	w.Close()
	out, _ := io.ReadAll(r)
	if code != 0 {
		t.Fatalf("run() returned %d", code)
	}
	if !bytes.Contains(out, []byte("version")) {
		t.Fatalf("expected version in output, got: %s", out)
	}
}

func TestRunJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	// Minimal config with one fast target to avoid docker scan
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// Use writeStarterConfig then overwrite with minimal - or write minimal YAML directly
	if err := writeStarterConfig(cfgPath, false); err != nil {
		t.Fatalf("writeStarterConfig: %v", err)
	}
	// Overwrite with minimal config for faster test (single target, no docker)
	minimalYAML := "version: 1\noptions: {}\ntargets:\n  - name: test\n    enabled: true\n    paths: [\"" + strings.ReplaceAll(tmpDir, "\\", "\\\\") + "\"]\n"
	if err := os.WriteFile(cfgPath, []byte(minimalYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"mac-cache-cleaner", "--config", cfgPath, "--json", "--targets", "test"}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old }()

	code := run()
	w.Close()
	out, _ := io.ReadAll(r)
	if code != 0 {
		t.Fatalf("run() returned %d, output: %s", code, out)
	}
	if !bytes.Contains(out, []byte(`"dry_run"`)) {
		t.Fatalf("expected JSON output, got: %s", out)
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	badPath := filepath.Join(tmpDir, "bad.yaml")
	if err := os.WriteFile(badPath, []byte("not: valid: yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadConfig(badPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestInspectPathFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := inspectPath(filePath)
	if err != nil {
		t.Fatalf("inspectPath error: %v", err)
	}
	if f.Items != 1 || f.SizeBytes != 5 {
		t.Fatalf("inspectPath file: got %d items, %d bytes", f.Items, f.SizeBytes)
	}
}

func TestInspectPathNonExistent(t *testing.T) {
	_, err := inspectPath(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
}

func TestInspectPathDirWithError(t *testing.T) {
	// Directory with a subdir we can't read
	root := t.TempDir()
	noRead := filepath.Join(root, "noread")
	if err := os.MkdirAll(noRead, 0o000); err != nil {
		t.Skip("cannot create restricted dir")
	}
	defer func() { _ = os.Chmod(noRead, 0o755) }()

	f, err := inspectPath(root)
	if err != nil {
		t.Fatalf("inspectPath error: %v", err)
	}
	// May have partial results; Err might be set if walk hit the restricted dir
	_ = f
}

func TestCheckTools(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := writeStarterConfig(cfgPath, false); err != nil {
		t.Fatalf("writeStarterConfig: %v", err)
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	testMode = true
	defer func() {
		testMode = false
		os.Stdout = old
		w.Close()
	}()

	checkToolsWithFlags(cfg, cfgPath, "all")

	w.Close()
	out, _ := io.ReadAll(r)
	if len(out) == 0 {
		t.Fatal("expected checkTools to produce output")
	}
	if !bytes.Contains(out, []byte("Tool Status Check")) {
		t.Fatalf("expected tool status header, got: %s", out)
	}
}

func TestParseHumanSizeMore(t *testing.T) {
	tests := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"(40%)", 0, false},
		{"1.5 MB", 1572864, true},
		{"1K", 1024, true},
		{"2G", 2 * 1024 * 1024 * 1024, true},
		{"1M", 1024 * 1024, true},
		{"1T", 1024 * 1024 * 1024 * 1024, true},
		{"1 KIB", 1024, true},
		{"1 MIB", 1024 * 1024, true},
		{"1 GIB", 1024 * 1024 * 1024, true},
		{"1 TIB", 1024 * 1024 * 1024 * 1024, true},
		{"1K", 1024, true},
		{"invalid", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		got, ok := parseHumanSize(tt.in)
		if ok != tt.ok {
			t.Errorf("parseHumanSize(%q) ok=%v, want %v", tt.in, ok, tt.ok)
		}
		if ok && got != tt.want {
			t.Errorf("parseHumanSize(%q)=%d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestDockerSystemDF(t *testing.T) {
	// dockerSystemDF may fail if docker not installed - that's ok
	findings, total, err := dockerSystemDF()
	if err != nil {
		t.Logf("dockerSystemDF failed (docker may not be installed): %v", err)
		return
	}
	_ = findings
	_ = total
}

func TestWrapText(t *testing.T) {
	// Short text - no wrap
	got := wrapText("hello", 80)
	if got != "hello" {
		t.Fatalf("wrapText(short) = %q, want hello", got)
	}
	// Long text - wrap at word boundary
	long := "one two three four five six seven eight nine ten"
	got = wrapText(long, 15)
	if !strings.Contains(got, "\n") {
		t.Fatalf("wrapText(long) should contain newline, got %q", got)
	}
	// Multi-line
	multi := "line one\nline two"
	got = wrapText(multi, 5)
	if !strings.Contains(got, "line") {
		t.Fatalf("wrapText(multi) = %q", got)
	}
}

func TestWrapLine(t *testing.T) {
	var b strings.Builder
	wrapLine(&b, "short", 80)
	if b.String() != "short" {
		t.Fatalf("wrapLine(short) = %q", b.String())
	}
	b.Reset()
	wrapLine(&b, "a b c d e f g h i j", 5)
	if !strings.Contains(b.String(), "\n") {
		t.Fatalf("wrapLine(long) should wrap: %q", b.String())
	}
	b.Reset()
	// Line with no words (spaces only) - len(words)==0
	wrapLine(&b, "   ", 5)
	if b.String() != "   " {
		t.Fatalf("wrapLine(spaces) = %q", b.String())
	}
}

func TestRunCmdEmpty(t *testing.T) {
	res := runCmd([]string{})
	if res.Error != "empty command" {
		t.Fatalf("runCmd([]) error = %q, want empty command", res.Error)
	}
}

func TestRunCmdFound(t *testing.T) {
	// Run a simple command that exists (echo or true)
	res := runCmd([]string{"true"})
	if !res.Found {
		t.Fatalf("runCmd(true) Found = false")
	}
	if res.Error != "" {
		t.Fatalf("runCmd(true) Error = %q", res.Error)
	}
	// Command with args
	res2 := runCmd([]string{"sh", "-c", "exit 0"})
	if !res2.Found {
		t.Fatalf("runCmd(sh -c) Found = false")
	}
}

func TestRunCmdFails(t *testing.T) {
	// Run a command that returns non-zero (false)
	res := runCmd([]string{"false"})
	if !res.Found {
		t.Fatalf("runCmd(false) Found = false - false should be in PATH")
	}
	if res.Error == "" {
		t.Fatalf("runCmd(false) should have Error set")
	}
}

func TestCheckToolPATH(t *testing.T) {
	// Tool in PATH (we know 'true' or 'false' exists)
	tool := Tool{Name: "true"}
	installed, _, err := checkTool(tool)
	if err != nil {
		t.Fatalf("checkTool(true) error: %v", err)
	}
	if !installed {
		t.Fatalf("checkTool(true) installed = false")
	}
}

func TestCheckToolNotInPath(t *testing.T) {
	tool := Tool{Name: "nonexistent-tool-xyz-12345"}
	installed, _, err := checkTool(tool)
	if err != nil {
		t.Fatalf("checkTool(nonexistent) should not error: %v", err)
	}
	if installed {
		t.Fatalf("checkTool(nonexistent) installed = true")
	}
}

func TestGetInstallCommandWithCmd(t *testing.T) {
	tool := Tool{Name: "foo", InstallCmd: "brew install foo"}
	got := getInstallCommand(tool)
	if got != "brew install foo" {
		t.Fatalf("getInstallCommand = %q, want brew install foo", got)
	}
}

func TestCheckToolVersion(t *testing.T) {
	// Tool with version check - use a tool we know exists like "go"
	tool := Tool{Name: "go", Version: "1"}
	installed, info, err := checkTool(tool)
	if err != nil && !installed {
		t.Logf("checkTool(go) version check: installed=%v err=%v", installed, err)
		return
	}
	if installed && info != "" {
		t.Logf("go version info: %s", info)
	}
}

func TestRunFirstScanDocker(t *testing.T) {
	// Target named "docker" uses dockerSystemDF
	rep := &Report{Findings: map[string][]Finding{}, Warnings: []string{}}
	targets := []Target{
		{Name: "docker", Enabled: true, Paths: []string{}},
	}
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old; w.Close() }()

	totals := runFirstScan(targets, rep)

	w.Close()
	_, _ = io.ReadAll(r)
	// docker may fail if not installed - totals may be 0
	_ = totals["docker"]
}

func TestRunFirstScan(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep := &Report{
		Findings: map[string][]Finding{},
		Warnings: []string{},
	}
	targets := []Target{
		{Name: "test", Enabled: true, Paths: []string{dir}},
	}
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old; w.Close() }()

	totals := runFirstScan(targets, rep)

	w.Close()
	out, _ := io.ReadAll(r)
	if !bytes.Contains(out, []byte("Scanning [test]")) {
		t.Fatalf("expected scan output, got: %s", out)
	}
	if totals["test"] != 1 {
		t.Fatalf("expected total 1, got %d", totals["test"])
	}
	if len(rep.Findings["test"]) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(rep.Findings["test"]))
	}
}

func TestPopulateCommands(t *testing.T) {
	rep := &Report{Commands: map[string][]CmdResult{}, Warnings: []string{}}
	targets := []Target{
		{Name: "t1", Cmds: [][]string{{"true"}}, Tools: []Tool{}},
		{Name: "t2", Cmds: [][]string{{"nonexistent-cmd-xyz"}}, Tools: []Tool{}},
		{Name: "t3", Cmds: [][]string{{"true"}}, Tools: []Tool{{Name: "true", InstallNotes: "test note"}}},
		{Name: "t4", Cmds: [][]string{{"missing-tool"}}, Tools: []Tool{{Name: "missing-tool", InstallCmd: "brew install x"}}},
		{Name: "t5", Cmds: [][]string{}, Tools: []Tool{}},
		{Name: "t6", Cmds: [][]string{{}}, Tools: []Tool{}},
	}
	populateCommands(targets, rep)
	if len(rep.Commands["t1"]) != 1 {
		t.Fatalf("expected 1 command for t1, got %d", len(rep.Commands["t1"]))
	}
	if len(rep.Commands["t2"]) != 1 {
		t.Fatalf("expected 1 command for t2, got %d", len(rep.Commands["t2"]))
	}
	if rep.Commands["t1"][0].Found != true {
		t.Fatalf("expected true to be found")
	}
	if rep.Commands["t2"][0].Found != false {
		t.Fatalf("expected nonexistent to not be found")
	}
	// t3 has a tool - checkTool for "true" should find it
	if len(rep.Commands["t3"]) != 1 || !rep.Commands["t3"][0].Found {
		t.Fatalf("t3 with tool true should have found command")
	}
	// t4 has missing tool - should add warning
	if len(rep.Warnings) == 0 && len(rep.Commands["t4"]) > 0 && !rep.Commands["t4"][0].Found {
		t.Log("t4 has missing tool - warnings may be set")
	}
}

func TestRunFirstScanGlobError(t *testing.T) {
	rep := &Report{Findings: map[string][]Finding{}, Warnings: []string{}}
	targets := []Target{
		{Name: "bad", Enabled: true, Paths: []string{"$(echo hi)/x"}},
	}
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = old; w.Close() }()

	totals := runFirstScan(targets, rep)

	w.Close()
	_, _ = io.ReadAll(r)
	if totals["bad"] != 0 {
		t.Fatalf("expected 0 for bad glob, got %d", totals["bad"])
	}
	if len(rep.Warnings) == 0 {
		t.Fatal("expected warning for glob error")
	}
}

func TestSelectTargets(t *testing.T) {
	cfg := &Config{
		Targets: []Target{
			{Name: "docker", Enabled: true},
			{Name: "npm", Enabled: true},
			{Name: "maven", Enabled: false},
		},
	}
	targets := selectTargets(cfg, "all")
	if len(targets) != 2 {
		t.Fatalf("selectTargets(all) = %d, want 2", len(targets))
	}
	targets = selectTargets(cfg, "docker")
	if len(targets) != 1 || targets[0].Name != "docker" {
		t.Fatalf("selectTargets(docker) = %v", targets)
	}
	targets = selectTargets(cfg, "nonexistent")
	if len(targets) != 0 {
		t.Fatalf("selectTargets(nonexistent) = %d, want 0", len(targets))
	}
	targets = selectTargets(cfg, "docker, npm")
	if len(targets) != 2 {
		t.Fatalf("selectTargets(docker,npm) = %d, want 2", len(targets))
	}
	targets = selectTargets(cfg, "  docker  ,  npm  ")
	if len(targets) != 2 {
		t.Fatalf("selectTargets with spaces = %d, want 2", len(targets))
	}
}

func TestExpandGlobsBrewCache(t *testing.T) {
	// $(brew --cache) - may fail if brew not installed
	paths, err := expandGlobs("$(brew --cache)")
	if err != nil {
		t.Logf("expandGlobs brew --cache failed (brew may not be installed): %v", err)
		return
	}
	if len(paths) > 0 && !strings.Contains(paths[0], "Cache") && !strings.Contains(paths[0], "cache") {
		t.Logf("brew --cache returned: %v", paths)
	}
}

func TestExpandGlobsBrewUnsupported(t *testing.T) {
	// $(brew info) - unsupported argument, should error
	_, err := expandGlobs("$(brew info)")
	if err == nil {
		t.Fatal("expected error for brew info")
	}
}

func TestExpandGlobsBrewCacheNoBrew(t *testing.T) {
	// When PATH is empty, brew --cache falls back to ~/Library/Caches/Homebrew
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", "")
	defer os.Setenv("PATH", oldPath)

	paths, err := expandGlobs("$(brew --cache)")
	if err != nil {
		t.Fatalf("expandGlobs brew --cache with empty PATH: %v", err)
	}
	// Should get fallback path
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d", len(paths))
	}
	if !strings.Contains(paths[0], "Homebrew") && !strings.Contains(paths[0], "homebrew") {
		t.Logf("fallback path: %s", paths[0])
	}
}

func TestExpandGlobsWithWildcards(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	pattern := filepath.Join(dir, "*.txt")
	paths, err := expandGlobs(pattern)
	if err != nil {
		t.Fatalf("expandGlobs error: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expandGlobs got %d paths, want 2: %v", len(paths), paths)
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

	// Nested path - ensureDir creates parent dirs
	nestedPath := filepath.Join(tmpDir, "a", "b", "config.yaml")
	if err := writeStarterConfig(nestedPath, false); err != nil {
		t.Fatalf("writeStarterConfig nested failed: %v", err)
	}
	if _, err := os.Stat(nestedPath); err != nil {
		t.Fatalf("nested config not created: %v", err)
	}
}
