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

func TestDefaultsPlaceTheExtraRows(t *testing.T) {
	c := Defaults()
	if want := []string{"autoMode"}; !reflect.DeepEqual(c.Widgets.Row3, want) {
		t.Errorf("Row3 = %v, want %v", c.Widgets.Row3, want)
	}
	if want := []string{"cache"}; !reflect.DeepEqual(c.Widgets.Row4, want) {
		t.Errorf("Row4 = %v, want %v", c.Widgets.Row4, want)
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
