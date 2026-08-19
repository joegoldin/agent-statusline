// Package config loads ~/.claude/statusline-config.json and merges it
// over compiled-in defaults. Unknown fields are tolerated. Missing or
// malformed files fall back to defaults but Load returns the error so
// callers can log to debug.
package config

import (
	"encoding/json"
	"errors"
	"os"
)

type Config struct {
	Padding                 int     `json:"padding"`
	RefreshInterval         int     `json:"refreshInterval"`
	ActivityRows            int     `json:"activityRows"`
	HideWhenIdle            bool    `json:"hideWhenIdle"`
	Widgets                 Widgets `json:"widgets"`
	GitCacheTTLSeconds      int     `json:"gitCacheTtlSeconds"`
	TranscriptWindowSeconds int     `json:"transcriptWindowSeconds"`
	BarWidth                int     `json:"barWidth"`
	SevenDayThreshold       int     `json:"sevenDayThreshold"`
	TokenFormat             string  `json:"tokenFormat"`
}

type Widgets struct {
	Row1 []string `json:"row1"`
	Row2 []string `json:"row2"`
	Row3 []string `json:"row3"`
	Row4 []string `json:"row4"`
	Hide []string `json:"hide"`
}

// Defaults returns a fresh Config with all fields set to documented defaults.
func Defaults() Config {
	return Config{
		Padding:         0,
		RefreshInterval: 1,
		ActivityRows:    4,
		HideWhenIdle:    true,
		Widgets: Widgets{
			// Row 1 — identity + budget (model/effort live together inside the
			// model widget; session clock sits right after git; account-usage
			// windows close out the row).
			Row1: []string{"model", "cwd", "git", "duration", "usage5h", "usage7d"},
			// Row 2 — conversation state (what's happening this session).
			Row2: []string{"context", "tokens", "burnRate", "voice", "compaction", "pr", "cost"},
			// Rows 3 and 4 each carry one widget that draws several figures of
			// its own, so they get a line rather than a slot: crowding them
			// into row 2 would push the conversation state off a narrow
			// terminal. Both hide themselves when their source is absent, and
			// an empty row costs no line.
			Row3: []string{"autoMode"},
			Row4: []string{"cache"},
			Hide: []string{},
		},
		TokenFormat:             "compact",
		GitCacheTTLSeconds:      5,
		TranscriptWindowSeconds: 300,
		BarWidth:                10,
		SevenDayThreshold:       50,
	}
}

// Load reads path and overlays it on Defaults. Missing file returns
// (Defaults(), nil). Malformed file returns (Defaults(), err).
func Load(path string) (Config, error) {
	c := Defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c, nil
		}
		return c, err
	}
	var overlay overlayConfig
	if err := json.Unmarshal(data, &overlay); err != nil {
		return c, err
	}
	overlay.applyTo(&c)
	return c, nil
}

type overlayConfig struct {
	Padding                 *int            `json:"padding"`
	RefreshInterval         *int            `json:"refreshInterval"`
	ActivityRows            *int            `json:"activityRows"`
	HideWhenIdle            *bool           `json:"hideWhenIdle"`
	Widgets                 *overlayWidgets `json:"widgets"`
	GitCacheTTLSeconds      *int            `json:"gitCacheTtlSeconds"`
	TranscriptWindowSeconds *int            `json:"transcriptWindowSeconds"`
	BarWidth                *int            `json:"barWidth"`
	SevenDayThreshold       *int            `json:"sevenDayThreshold"`
	TokenFormat             *string         `json:"tokenFormat"`
}

type overlayWidgets struct {
	Row1 *[]string `json:"row1"`
	Row2 *[]string `json:"row2"`
	Row3 *[]string `json:"row3"`
	Row4 *[]string `json:"row4"`
	Hide *[]string `json:"hide"`
}

func (o overlayConfig) applyTo(c *Config) {
	if o.Padding != nil {
		c.Padding = *o.Padding
	}
	if o.RefreshInterval != nil {
		c.RefreshInterval = *o.RefreshInterval
	}
	if o.ActivityRows != nil {
		c.ActivityRows = *o.ActivityRows
	}
	if o.HideWhenIdle != nil {
		c.HideWhenIdle = *o.HideWhenIdle
	}
	if o.Widgets != nil {
		if o.Widgets.Row1 != nil {
			c.Widgets.Row1 = *o.Widgets.Row1
		}
		if o.Widgets.Row2 != nil {
			c.Widgets.Row2 = *o.Widgets.Row2
		}
		if o.Widgets.Row3 != nil {
			c.Widgets.Row3 = *o.Widgets.Row3
		}
		if o.Widgets.Row4 != nil {
			c.Widgets.Row4 = *o.Widgets.Row4
		}
		if o.Widgets.Hide != nil {
			c.Widgets.Hide = *o.Widgets.Hide
		}
	}
	if o.GitCacheTTLSeconds != nil {
		c.GitCacheTTLSeconds = *o.GitCacheTTLSeconds
	}
	if o.TranscriptWindowSeconds != nil {
		c.TranscriptWindowSeconds = *o.TranscriptWindowSeconds
	}
	if o.BarWidth != nil {
		c.BarWidth = *o.BarWidth
	}
	if o.SevenDayThreshold != nil {
		c.SevenDayThreshold = *o.SevenDayThreshold
	}
	if o.TokenFormat != nil {
		c.TokenFormat = *o.TokenFormat
	}
}

// ResolvePath returns the effective config path, honoring CLAUDE_STATUSLINE_CONFIG
// and CLAUDE_CONFIG_DIR. Fallback: $HOME/.claude/statusline-config.json.
func ResolvePath() string {
	if p := os.Getenv("CLAUDE_STATUSLINE_CONFIG"); p != "" {
		return p
	}
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return d + "/statusline-config.json"
	}
	home, _ := os.UserHomeDir()
	return home + "/.claude/statusline-config.json"
}
