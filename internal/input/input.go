// Package input decodes Claude Code's statusline stdin JSON.
// Optional fields use pointers so callers can distinguish "absent" from "zero".
package input

import (
	"encoding/json"
	"io"
)

type Status struct {
	CWD            string         `json:"cwd"`
	SessionID      string         `json:"session_id"`
	SessionName    string         `json:"session_name"`
	TranscriptPath string         `json:"transcript_path"`
	Version        string         `json:"version"`
	Model          Model          `json:"model"`
	Workspace      Workspace      `json:"workspace"`
	Cost           *Cost          `json:"cost"`
	ContextWindow  *ContextWindow `json:"context_window"`
	RateLimits     *RateLimits    `json:"rate_limits"`
	Effort         *Effort        `json:"effort"`
	PR             *PR            `json:"pr"`
	Worktree       *WorktreeInfo  `json:"worktree"`
	Vim            *Vim           `json:"vim"`
}

type Model struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type Workspace struct {
	CurrentDir  string `json:"current_dir"`
	ProjectDir  string `json:"project_dir"`
	GitWorktree string `json:"git_worktree"`
}

type Cost struct {
	TotalCostUSD       float64 `json:"total_cost_usd"`
	TotalDurationMS    int64   `json:"total_duration_ms"`
	TotalAPIDurationMS int64   `json:"total_api_duration_ms"`
}

type ContextWindow struct {
	ContextWindowSize   int           `json:"context_window_size"`
	UsedPercentage      *float64      `json:"used_percentage"`
	RemainingPercentage *float64      `json:"remaining_percentage"`
	TotalInputTokens    int           `json:"total_input_tokens"`
	TotalOutputTokens   int           `json:"total_output_tokens"`
	CurrentUsage        *CurrentUsage `json:"current_usage"`
}

type CurrentUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

type RateLimits struct {
	FiveHour *Window `json:"five_hour"`
	SevenDay *Window `json:"seven_day"`
}

type Window struct {
	UsedPercentage float64 `json:"used_percentage"`
	ResetsAt       int64   `json:"resets_at"`
}

type Effort struct {
	Level string `json:"level"`
}

type PR struct {
	Number      int    `json:"number"`
	URL         string `json:"url"`
	ReviewState string `json:"review_state"`
}

type WorktreeInfo struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	Branch         string `json:"branch"`
	OriginalBranch string `json:"original_branch"`
}

type Vim struct {
	Mode string `json:"mode"`
}

// Decode reads stdin JSON into a Status.
func Decode(r io.Reader) (Status, error) {
	var s Status
	dec := json.NewDecoder(r)
	if err := dec.Decode(&s); err != nil {
		return s, err
	}
	return s, nil
}
