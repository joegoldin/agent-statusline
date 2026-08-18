package input

import (
	"encoding/json"
	"io"
)

// PiStatus is the wire format the agent-statusline pi extension emits. It
// mirrors what pi natively exposes to an extension; every derived value
// (percentages, workspace fields) is computed here rather than in TypeScript,
// so translation stays under test.
type PiStatus struct {
	Harness       string      `json:"harness"`
	CWD           string      `json:"cwd"`
	SessionID     string      `json:"session_id"`
	SessionName   string      `json:"session_name"`
	SessionPath   string      `json:"session_path"`
	ProjectDir    string      `json:"project_dir"`
	Model         Model       `json:"model"`
	ThinkingLevel string      `json:"thinking_level"`
	Context       *PiContext  `json:"context"`
	CostUSD       *float64    `json:"cost_usd"`
	DurationMS    int64       `json:"duration_ms"`
	APIDurationMS int64       `json:"api_duration_ms"`
	RateLimits    *RateLimits `json:"rate_limits"`
	PR            *PR         `json:"pr"`
	Version       string      `json:"version"`
}

// PiContext is pi's raw token accounting. Percentages are deliberately absent:
// deriving them is the Go side's job so both harnesses agree on the formula.
type PiContext struct {
	WindowSize          int `json:"window_size"`
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheReadTokens     int `json:"cache_read_tokens"`
	CacheCreationTokens int `json:"cache_creation_tokens"`
}

// DecodePi reads the pi extension's JSON and projects it onto the canonical
// Status every widget already understands.
func DecodePi(r io.Reader) (Status, error) {
	var p PiStatus
	if err := json.NewDecoder(r).Decode(&p); err != nil {
		return Status{}, err
	}

	projectDir := p.ProjectDir
	if projectDir == "" {
		projectDir = p.CWD
	}

	s := Status{
		CWD:            p.CWD,
		SessionID:      p.SessionID,
		SessionName:    p.SessionName,
		TranscriptPath: p.SessionPath,
		Version:        p.Version,
		Model:          p.Model,
		Workspace: Workspace{
			CurrentDir: p.CWD,
			ProjectDir: projectDir,
		},
		RateLimits: p.RateLimits,
		PR:         p.PR,
	}

	if p.ThinkingLevel != "" {
		s.Effort = &Effort{Level: p.ThinkingLevel}
	}

	if p.CostUSD != nil || p.DurationMS != 0 {
		c := Cost{TotalDurationMS: p.DurationMS, TotalAPIDurationMS: p.APIDurationMS}
		if p.CostUSD != nil {
			c.TotalCostUSD = *p.CostUSD
		}
		s.Cost = &c
	}

	if p.Context != nil {
		s.ContextWindow = piContextWindow(*p.Context)
	}

	return s, nil
}

// piContextWindow mirrors Claude Code's accounting: used context is input plus
// both cache figures, and output tokens are excluded. Percentages stay nil when
// the window size is unknown, so widgets hide rather than render a bogus 0%.
func piContextWindow(c PiContext) *ContextWindow {
	cw := &ContextWindow{
		ContextWindowSize: c.WindowSize,
		TotalInputTokens:  c.InputTokens,
		TotalOutputTokens: c.OutputTokens,
		CurrentUsage: &CurrentUsage{
			InputTokens:              c.InputTokens,
			OutputTokens:             c.OutputTokens,
			CacheReadInputTokens:     c.CacheReadTokens,
			CacheCreationInputTokens: c.CacheCreationTokens,
		},
	}
	if c.WindowSize > 0 {
		used := float64(c.InputTokens+c.CacheReadTokens+c.CacheCreationTokens) /
			float64(c.WindowSize) * 100
		remaining := 100 - used
		cw.UsedPercentage = &used
		cw.RemainingPercentage = &remaining
	}
	return cw
}
