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
