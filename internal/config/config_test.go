package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaults(t *testing.T) {
	c := Defaults()
	if c.ActivityRows != 4 {
		t.Errorf("ActivityRows = %d, want 4", c.ActivityRows)
	}
	if !c.HideWhenIdle {
		t.Errorf("HideWhenIdle = false, want true")
	}
	if c.BarWidth != 10 {
		t.Errorf("BarWidth = %d, want 10", c.BarWidth)
	}
	if c.GitCacheTTLSeconds != 5 {
		t.Errorf("GitCacheTTLSeconds = %d, want 5", c.GitCacheTTLSeconds)
	}
	if c.TranscriptWindowSeconds != 300 {
		t.Errorf("TranscriptWindowSeconds = %d, want 300", c.TranscriptWindowSeconds)
	}
	if c.SevenDayThreshold != 50 {
		t.Errorf("SevenDayThreshold = %d, want 50", c.SevenDayThreshold)
	}
	if c.TokenFormat != "compact" {
		t.Errorf("TokenFormat = %q, want compact", c.TokenFormat)
	}
	wantRow1 := []string{"model", "cwd", "git", "duration", "usage5h", "usage7d"}
	if !reflect.DeepEqual(c.Widgets.Row1, wantRow1) {
		t.Errorf("Row1 = %v, want %v", c.Widgets.Row1, wantRow1)
	}
}

func TestLoadMergesOverDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statusline-config.json")
	body := `{"activityRows":1,"widgets":{"row1":["model"]},"barWidth":4}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.ActivityRows != 1 {
		t.Errorf("ActivityRows = %d, want 1", c.ActivityRows)
	}
	if c.BarWidth != 4 {
		t.Errorf("BarWidth = %d, want 4", c.BarWidth)
	}
	if c.GitCacheTTLSeconds != 5 {
		t.Errorf("GitCacheTTLSeconds = %d, want 5", c.GitCacheTTLSeconds)
	}
	if !reflect.DeepEqual(c.Widgets.Row1, []string{"model"}) {
		t.Errorf("Row1 = %v", c.Widgets.Row1)
	}
	if len(c.Widgets.Row2) == 0 {
		t.Errorf("Row2 should have fallen back to defaults, got empty")
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.ActivityRows != 4 {
		t.Errorf("ActivityRows = %d, want 4", c.ActivityRows)
	}
}

func TestLoadMalformedFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	_ = os.WriteFile(path, []byte("{not json"), 0o644)
	c, err := Load(path)
	if err == nil {
		t.Errorf("expected error for malformed JSON")
	}
	if c.ActivityRows != 4 {
		t.Errorf("ActivityRows should still default, got %d", c.ActivityRows)
	}
}

func TestDefaultsPutBothExtraWidgetsOnOneRow(t *testing.T) {
	// One multi-figure widget per row left a mostly empty line under a mostly
	// empty line. Row 4 stays as a spare, not a leftover.
	c := Defaults()
	if want := []string{"autoMode", "cache"}; !reflect.DeepEqual(c.Widgets.Row3, want) {
		t.Errorf("Row3 = %v, want %v", c.Widgets.Row3, want)
	}
	if len(c.Widgets.Row4) != 0 {
		t.Errorf("Row4 = %v, want empty", c.Widgets.Row4)
	}
}

func TestLoadEmptiesARowWhenAsked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "statusline-config.json")
	// An empty list is how a row is turned off, so it has to survive the merge
	// rather than being mistaken for "unset" and replaced by the default.
	if err := os.WriteFile(path, []byte(`{"widgets":{"row3":[],"row4":["cache"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Widgets.Row3) != 0 {
		t.Errorf("Row3 = %v, want empty", c.Widgets.Row3)
	}
	if !reflect.DeepEqual(c.Widgets.Row4, []string{"cache"}) {
		t.Errorf("Row4 = %v", c.Widgets.Row4)
	}
}

func TestResolvePathPrefersTheAgentNeutralVariable(t *testing.T) {
	// pi's wrapper exports AGENT_STATUSLINE_CONFIG. Nothing read it, so every
	// jailed pi session fell through to ~/.claude, which the jail does not
	// bind, and rendered Defaults().
	t.Setenv("AGENT_STATUSLINE_CONFIG", "/agent/config.json")
	t.Setenv("CLAUDE_STATUSLINE_CONFIG", "/claude/config.json")
	t.Setenv("CLAUDE_CONFIG_DIR", "/claude-dir")
	if got := ResolvePath(); got != "/agent/config.json" {
		t.Errorf("ResolvePath() = %q, want /agent/config.json", got)
	}
}

func TestResolvePathKeepsTheClaudeVariables(t *testing.T) {
	t.Setenv("AGENT_STATUSLINE_CONFIG", "")
	t.Setenv("CLAUDE_STATUSLINE_CONFIG", "/claude/config.json")
	if got := ResolvePath(); got != "/claude/config.json" {
		t.Errorf("ResolvePath() = %q, want /claude/config.json", got)
	}
	t.Setenv("CLAUDE_STATUSLINE_CONFIG", "")
	t.Setenv("CLAUDE_CONFIG_DIR", "/claude-dir")
	if got := ResolvePath(); got != "/claude-dir/statusline-config.json" {
		t.Errorf("ResolvePath() = %q, want /claude-dir/statusline-config.json", got)
	}
}
