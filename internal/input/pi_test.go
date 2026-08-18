package input

import (
	"strings"
	"testing"
)

const samplePiJSON = `{
  "harness": "pi",
  "cwd": "/home/joe/Development/pi-nix",
  "session_id": "pi-abc-123",
  "session_name": "fork-the-flake",
  "session_path": "/home/joe/.pi/agent/sessions/pi-abc-123.jsonl",
  "model": {"id": "gpt-5.6-sol", "display_name": "Sol"},
  "thinking_level": "xhigh",
  "context": {
    "window_size": 400000,
    "input_tokens": 120000,
    "output_tokens": 8000,
    "cache_read_tokens": 90000,
    "cache_creation_tokens": 4000
  },
  "cost_usd": 0.42,
  "duration_ms": 330000
}`

func TestDecodePi(t *testing.T) {
	s, err := DecodePi(strings.NewReader(samplePiJSON))
	if err != nil {
		t.Fatalf("DecodePi: %v", err)
	}
	if s.CWD != "/home/joe/Development/pi-nix" {
		t.Errorf("CWD = %q", s.CWD)
	}
	if s.SessionID != "pi-abc-123" || s.SessionName != "fork-the-flake" {
		t.Errorf("session = %q / %q", s.SessionID, s.SessionName)
	}
	if s.Model.ID != "gpt-5.6-sol" || s.Model.DisplayName != "Sol" {
		t.Errorf("model = %+v", s.Model)
	}
	if s.Workspace.CurrentDir != s.CWD {
		t.Errorf("Workspace.CurrentDir = %q, want %q", s.Workspace.CurrentDir, s.CWD)
	}
	if s.TranscriptPath != "/home/joe/.pi/agent/sessions/pi-abc-123.jsonl" {
		t.Errorf("TranscriptPath = %q", s.TranscriptPath)
	}
	if s.Effort == nil || s.Effort.Level != "xhigh" {
		t.Errorf("Effort = %+v", s.Effort)
	}
}

func TestDecodePiComputesContextPercentages(t *testing.T) {
	s, err := DecodePi(strings.NewReader(samplePiJSON))
	if err != nil {
		t.Fatalf("DecodePi: %v", err)
	}
	cw := s.ContextWindow
	if cw == nil {
		t.Fatal("ContextWindow is nil")
	}
	if cw.ContextWindowSize != 400000 {
		t.Errorf("ContextWindowSize = %d", cw.ContextWindowSize)
	}
	if cw.UsedPercentage == nil || *cw.UsedPercentage < 53.4 || *cw.UsedPercentage > 53.6 {
		t.Errorf("UsedPercentage = %v, want ~53.5", cw.UsedPercentage)
	}
	if cw.RemainingPercentage == nil || *cw.RemainingPercentage < 46.4 || *cw.RemainingPercentage > 46.6 {
		t.Errorf("RemainingPercentage = %v, want ~46.5", cw.RemainingPercentage)
	}
	if cw.TotalInputTokens != 120000 || cw.TotalOutputTokens != 8000 {
		t.Errorf("totals = %d / %d", cw.TotalInputTokens, cw.TotalOutputTokens)
	}
	if cw.CurrentUsage == nil || cw.CurrentUsage.CacheReadInputTokens != 90000 {
		t.Errorf("CurrentUsage = %+v", cw.CurrentUsage)
	}
}

func TestDecodePiCost(t *testing.T) {
	s, err := DecodePi(strings.NewReader(samplePiJSON))
	if err != nil {
		t.Fatalf("DecodePi: %v", err)
	}
	if s.Cost == nil || s.Cost.TotalCostUSD != 0.42 {
		t.Errorf("Cost = %+v", s.Cost)
	}
	if s.Cost.TotalDurationMS != 330000 {
		t.Errorf("TotalDurationMS = %d", s.Cost.TotalDurationMS)
	}
}

func TestDecodePiZeroWindowDoesNotDivideByZero(t *testing.T) {
	raw := `{"harness":"pi","cwd":"/x","context":{"window_size":0,"input_tokens":5}}`
	s, err := DecodePi(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("DecodePi: %v", err)
	}
	if s.ContextWindow == nil {
		t.Fatal("ContextWindow is nil")
	}
	if s.ContextWindow.UsedPercentage != nil {
		t.Errorf("UsedPercentage = %v, want nil when window size is 0", s.ContextWindow.UsedPercentage)
	}
}

func TestDecodePiOmitsRateLimitsWhenAbsent(t *testing.T) {
	s, err := DecodePi(strings.NewReader(samplePiJSON))
	if err != nil {
		t.Fatalf("DecodePi: %v", err)
	}
	if s.RateLimits != nil {
		t.Errorf("RateLimits = %+v, want nil on non-Anthropic auth", s.RateLimits)
	}
}

func TestDecodePiPassesThroughRateLimits(t *testing.T) {
	raw := `{"harness":"pi","cwd":"/x","rate_limits":{
	  "five_hour":{"used_percentage":12.5,"resets_at":1748260800}}}`
	s, err := DecodePi(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("DecodePi: %v", err)
	}
	if s.RateLimits == nil || s.RateLimits.FiveHour == nil {
		t.Fatalf("RateLimits = %+v", s.RateLimits)
	}
	if s.RateLimits.FiveHour.UsedPercentage != 12.5 {
		t.Errorf("FiveHour.UsedPercentage = %v", s.RateLimits.FiveHour.UsedPercentage)
	}
}
