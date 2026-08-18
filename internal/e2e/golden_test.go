package e2e

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite .golden files with current output")

type fixture struct {
	name  string
	width string
}

func TestGolden(t *testing.T) {
	tests := []fixture{
		{"idle", "80"},
		{"full", "120"},
		{"narrow", "40"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runGolden(t, tc)
		})
	}
}

func runGolden(t *testing.T, tc fixture) {
	t.Helper()
	stdinPath := filepath.Join("testdata", tc.name+".json")
	goldenPath := filepath.Join("testdata", tc.name+".golden")
	stdin, err := os.ReadFile(stdinPath)
	if err != nil {
		t.Fatal(err)
	}
	binPath := buildBinary(t)
	cmd := exec.Command(binPath)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Env = append(os.Environ(),
		"CLAUDE_STATUSLINE_WIDTH="+tc.width,
		"CLAUDE_STATUSLINE_CONFIG=/dev/null",
		// Pin "now" so countdowns are deterministic across runs.
		// 1748260800 = 2025-05-26 12:00:00 UTC.
		"CLAUDE_STATUSLINE_NOW=1748260800",
		"HOME=/tmp/claude-statusline-test-home",
		"CLAUDE_CONFIG_DIR=/tmp/claude-statusline-test-home/.claude",
		"GIT_CONFIG_GLOBAL=/dev/null",
	)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("binary failed: %v", err)
	}
	if *update {
		_ = os.MkdirAll(filepath.Dir(goldenPath), 0o755)
		if err := os.WriteFile(goldenPath, out, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("missing golden file %q (run with -update to create it): %v", goldenPath, err)
	}
	if !bytes.Equal(out, want) {
		t.Errorf("output mismatch.\n--- got ---\n%s\n--- want ---\n%s", visualise(out), visualise(want))
	}
}

var binaryOnce struct {
	path string
	err  error
}

func buildBinary(t *testing.T) string {
	t.Helper()
	if binaryOnce.path != "" || binaryOnce.err != nil {
		if binaryOnce.err != nil {
			t.Fatal(binaryOnce.err)
		}
		return binaryOnce.path
	}
	tmp := filepath.Join(os.TempDir(), "claude-statusline-e2e-bin")
	cmd := exec.Command("go", "build", "-o", tmp, "../../cmd/agent-statusline")
	if out, err := cmd.CombinedOutput(); err != nil {
		binaryOnce.err = err
		t.Fatalf("go build: %v\n%s", err, out)
	}
	binaryOnce.path = tmp
	return tmp
}

// visualise replaces ANSI escapes with placeholder tokens for readable diffs.
func visualise(b []byte) string {
	s := string(b)
	s = strings.ReplaceAll(s, "\x1b", "<ESC>")
	return s
}
