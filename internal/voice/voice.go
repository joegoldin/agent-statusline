// Package voice reads Claude Code's `voice` settings from the layered
// configuration files. Returns the highest-priority override (project
// local > project > user local > user). Returns nil only when none of
// the candidate files exist.
package voice

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Config struct {
	Enabled bool
	Mode    string
}

type Reader struct {
	UserDir string
}

// NewReader returns a Reader with UserDir resolved from CLAUDE_CONFIG_DIR,
// falling back to $HOME/.claude.
func NewReader() *Reader {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return &Reader{UserDir: d}
	}
	home, _ := os.UserHomeDir()
	return &Reader{UserDir: filepath.Join(home, ".claude")}
}

// Read returns the effective voice config for cwd, or nil if no candidate
// settings files exist.
func (r *Reader) Read(cwd string) *Config {
	candidates := []string{
		filepath.Join(cwd, ".claude", "settings.local.json"),
		filepath.Join(cwd, ".claude", "settings.json"),
		filepath.Join(r.UserDir, "settings.local.json"),
		filepath.Join(r.UserDir, "settings.json"),
	}
	anyExisted := false
	for _, p := range candidates {
		layer, existed := readLayer(p)
		if existed {
			anyExisted = true
		}
		if layer != nil {
			return layer
		}
	}
	if anyExisted {
		return &Config{}
	}
	return nil
}

type voiceField struct {
	Voice *struct {
		Enabled *bool   `json:"enabled"`
		Mode    *string `json:"mode"`
	} `json:"voice"`
}

func readLayer(path string) (*Config, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false
		}
		return nil, true
	}
	var parsed voiceField
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, true
	}
	if parsed.Voice == nil || parsed.Voice.Enabled == nil {
		return nil, true
	}
	cfg := &Config{Enabled: *parsed.Voice.Enabled}
	if parsed.Voice.Mode != nil {
		cfg.Mode = *parsed.Voice.Mode
	}
	return cfg, true
}
