package cachestats

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `{
  "version": 6,
  "sessions": {"abc": {"openai-codex/gpt-5.6-sol": {"totalRequests": 1}}},
  "totalsByModel": {
    "openai-codex/gpt-5.6-sol": {
      "day": "2026-08-19",
      "totalRequests": 41,
      "hitRequests": 35,
      "cachedInputTokens": 1690000,
      "cacheWriteInputTokens": 12000,
      "totalInputTokens": 2130000
    },
    "anthropic/claude-opus-4-8": {
      "day": "2026-08-19",
      "totalRequests": 0,
      "hitRequests": 0,
      "cachedInputTokens": 0,
      "cacheWriteInputTokens": 0,
      "totalInputTokens": 0
    }
  },
  "legacyFamily": {}
}`

func writeStats(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, FileName)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadReadsTotalsByModel(t *testing.T) {
	s, err := Load(writeStats(t, sample))
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s.Lookup("openai-codex", "gpt-5.6-sol")
	if !ok {
		t.Fatal("expected totals for the active model")
	}
	want := Totals{
		Day:                   "2026-08-19",
		TotalRequests:         41,
		HitRequests:           35,
		CachedInputTokens:     1_690_000,
		CacheWriteInputTokens: 12_000,
		TotalInputTokens:      2_130_000,
	}
	if got != want {
		t.Errorf("Lookup = %+v, want %+v", got, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), FileName))
	if err == nil {
		t.Error("expected an error for a missing file")
	}
	if s != nil {
		t.Errorf("expected no stats, got %+v", s)
	}
}

func TestLoadMalformedJSON(t *testing.T) {
	s, err := Load(writeStats(t, `{"totalsByModel": {`))
	if err == nil {
		t.Error("expected an error for malformed JSON")
	}
	if s != nil {
		t.Errorf("expected no stats, got %+v", s)
	}
}

func TestLookupTable(t *testing.T) {
	s, err := Load(writeStats(t, sample))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		provider string
		id       string
		want     bool
	}{
		{"active model", "openai-codex", "gpt-5.6-sol", true},
		{"other provider's model", "anthropic", "claude-opus-4-8", true},
		{"model absent from the file", "google", "gemini-3-pro", false},
		{"id without its provider", "", "gpt-5.6-sol", false},
		{"provider from another vendor", "openrouter", "gpt-5.6-sol", false},
	}
	for _, tc := range tests {
		if _, ok := s.Lookup(tc.provider, tc.id); ok != tc.want {
			t.Errorf("%s: Lookup(%q, %q) ok = %v, want %v", tc.name, tc.provider, tc.id, ok, tc.want)
		}
	}
}

func TestLookupOnNilStats(t *testing.T) {
	var s *Stats
	if _, ok := s.Lookup("openai-codex", "gpt-5.6-sol"); ok {
		t.Error("nil stats reported a hit")
	}
}

func TestPathHonoursPiCodingAgentDir(t *testing.T) {
	t.Setenv("PI_CODING_AGENT_DIR", "/tmp/pi-agent")
	if got, want := Path(), filepath.Join("/tmp/pi-agent", FileName); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("HOME", "/tmp/home")
	if got, want := Path(), filepath.Join("/tmp/home", ".pi", "agent", FileName); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}
