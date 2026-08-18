package voice

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadProjectLocalWins(t *testing.T) {
	cwd := t.TempDir()
	user := t.TempDir()

	writeJSON(t, filepath.Join(user, "settings.json"),
		`{"voice":{"enabled":false}}`)
	writeJSON(t, filepath.Join(cwd, ".claude", "settings.json"),
		`{"voice":{"enabled":true,"mode":"toggle"}}`)
	writeJSON(t, filepath.Join(cwd, ".claude", "settings.local.json"),
		`{"voice":{"enabled":true,"mode":"hold"}}`)

	r := &Reader{UserDir: user}
	cfg := r.Read(cwd)
	if cfg == nil {
		t.Fatal("expected config")
	}
	if !cfg.Enabled {
		t.Errorf("Enabled = false, want true")
	}
	if cfg.Mode != "hold" {
		t.Errorf("Mode = %q, want hold", cfg.Mode)
	}
}

func TestReadFallsThroughToUserSettings(t *testing.T) {
	cwd := t.TempDir()
	user := t.TempDir()
	writeJSON(t, filepath.Join(user, "settings.json"),
		`{"voice":{"enabled":true,"mode":"toggle"}}`)
	r := &Reader{UserDir: user}
	cfg := r.Read(cwd)
	if cfg == nil || !cfg.Enabled || cfg.Mode != "toggle" {
		t.Errorf("got %+v", cfg)
	}
}

func TestReadNoFilesReturnsNil(t *testing.T) {
	cwd := t.TempDir()
	user := t.TempDir()
	r := &Reader{UserDir: user}
	if cfg := r.Read(cwd); cfg != nil {
		t.Errorf("expected nil, got %+v", cfg)
	}
}

func TestReadFilesExistButNoVoiceFieldReturnsDisabled(t *testing.T) {
	cwd := t.TempDir()
	user := t.TempDir()
	writeJSON(t, filepath.Join(user, "settings.json"), `{"theme":"dark"}`)
	r := &Reader{UserDir: user}
	cfg := r.Read(cwd)
	if cfg == nil {
		t.Fatal("expected non-nil")
	}
	if cfg.Enabled {
		t.Errorf("Enabled = true, want false (file exists but no voice field)")
	}
}

func TestReadMalformedFileFallsThrough(t *testing.T) {
	cwd := t.TempDir()
	user := t.TempDir()
	writeJSON(t, filepath.Join(cwd, ".claude", "settings.json"), "{not json")
	writeJSON(t, filepath.Join(user, "settings.json"),
		`{"voice":{"enabled":true,"mode":"hold"}}`)
	r := &Reader{UserDir: user}
	cfg := r.Read(cwd)
	if cfg == nil || !cfg.Enabled || cfg.Mode != "hold" {
		t.Errorf("got %+v", cfg)
	}
}

func writeJSON(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
