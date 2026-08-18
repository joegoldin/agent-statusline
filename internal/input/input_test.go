package input

import (
	"strings"
	"testing"
)

const sampleJSON = `{
  "cwd": "/home/joe/projects/claude-nix",
  "session_id": "abc-123",
  "session_name": "morning-hack",
  "transcript_path": "/home/joe/.claude/projects/abc/transcript.jsonl",
  "model": {"id": "claude-opus-4-7", "display_name": "Opus"},
  "workspace": {
    "current_dir": "/home/joe/projects/claude-nix",
    "project_dir": "/home/joe/projects/claude-nix",
    "git_worktree": "feature-statusline"
  },
  "cost": {"total_cost_usd": 1.42, "total_duration_ms": 330000},
  "context_window": {
    "context_window_size": 200000,
    "used_percentage": 47.2,
    "current_usage": {"input_tokens": 30000, "cache_read_input_tokens": 60000}
  },
  "rate_limits": {
    "five_hour": {"used_percentage": 32.5, "resets_at": 1738425600},
    "seven_day": {"used_percentage": 71.0, "resets_at": 1738857600}
  },
  "effort": {"level": "high"},
  "pr": {"number": 42, "url": "https://github.com/x/y/pull/42", "review_state": "pending"}
}`

func TestDecode(t *testing.T) {
	s, err := Decode(strings.NewReader(sampleJSON))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if s.Model.DisplayName != "Opus" {
		t.Errorf("display_name = %q", s.Model.DisplayName)
	}
	if s.Model.ID != "claude-opus-4-7" {
		t.Errorf("id = %q", s.Model.ID)
	}
	if s.CWD != "/home/joe/projects/claude-nix" {
		t.Errorf("cwd = %q", s.CWD)
	}
	if s.SessionID != "abc-123" {
		t.Errorf("session_id = %q", s.SessionID)
	}
	if s.SessionName != "morning-hack" {
		t.Errorf("session_name = %q", s.SessionName)
	}
	if s.Workspace.GitWorktree != "feature-statusline" {
		t.Errorf("worktree = %q", s.Workspace.GitWorktree)
	}
	if s.Cost == nil || s.Cost.TotalCostUSD != 1.42 {
		t.Errorf("cost = %+v", s.Cost)
	}
	if s.Cost.TotalDurationMS != 330000 {
		t.Errorf("duration = %d", s.Cost.TotalDurationMS)
	}
	if s.ContextWindow == nil || s.ContextWindow.UsedPercentage == nil || *s.ContextWindow.UsedPercentage != 47.2 {
		t.Errorf("used_percentage = %+v", s.ContextWindow)
	}
	if s.RateLimits == nil || s.RateLimits.FiveHour == nil || s.RateLimits.FiveHour.UsedPercentage != 32.5 {
		t.Errorf("five_hour = %+v", s.RateLimits)
	}
	if s.Effort == nil || s.Effort.Level != "high" {
		t.Errorf("effort = %+v", s.Effort)
	}
	if s.PR == nil || s.PR.Number != 42 {
		t.Errorf("pr = %+v", s.PR)
	}
}

func TestDecodeMissingOptionalFields(t *testing.T) {
	minimal := `{"cwd":"/x","session_id":"s","model":{"display_name":"Opus"}}`
	s, err := Decode(strings.NewReader(minimal))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if s.PR != nil {
		t.Errorf("PR should be nil, got %+v", s.PR)
	}
	if s.RateLimits != nil {
		t.Errorf("RateLimits should be nil, got %+v", s.RateLimits)
	}
}

func TestDecodeMalformed(t *testing.T) {
	if _, err := Decode(strings.NewReader("{garbage")); err == nil {
		t.Errorf("expected error for malformed JSON")
	}
}
