package layout

import "testing"

func TestDetectWidthEnvOverride(t *testing.T) {
	t.Setenv("CLAUDE_STATUSLINE_WIDTH", "120")
	if got := DetectWidth(); got != 120 {
		t.Errorf("got %d, want 120", got)
	}
}

func TestDetectWidthEnvOverrideInvalid(t *testing.T) {
	t.Setenv("CLAUDE_STATUSLINE_WIDTH", "garbage")
	if got := DetectWidth(); got <= 0 {
		t.Errorf("got non-positive width %d", got)
	}
}
