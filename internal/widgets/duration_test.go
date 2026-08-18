package widgets

import (
	"strings"
	"testing"
	"time"

	"github.com/joegoldin/agent-statusline/internal/input"
)

func TestFormatDurationCoarse(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "<1m"},
		{59 * time.Second, "<1m"},
		{60 * time.Second, "1m"},
		{90 * time.Second, "1m"},
		{5*time.Minute + 30*time.Second, "5m"},
		{time.Hour + time.Minute, "1h1m"},
		{2*24*time.Hour + 3*time.Hour, "2d3h"},
	}
	for _, tc := range tests {
		if got := formatDurationCoarse(tc.d); got != tc.want {
			t.Errorf("formatDurationCoarse(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestDurationMinuteGranularity(t *testing.T) {
	tests := []struct {
		ms   int64
		want string
	}{
		{30_000, "<1m"},
		{330_000, "5m"},
		{3_660_000, "1h1m"},
		{0, ""},
	}
	for _, tc := range tests {
		w := &Duration{}
		ctx := &Context{Status: input.Status{Cost: &input.Cost{TotalDurationMS: tc.ms}}}
		out, vis := w.Render(ctx)
		if tc.want == "" {
			if vis {
				t.Errorf("ms=%d: expected hidden", tc.ms)
			}
			continue
		}
		if !vis {
			t.Errorf("ms=%d: expected visible", tc.ms)
			continue
		}
		if !strings.Contains(out, tc.want) {
			t.Errorf("ms=%d: out=%q want %q", tc.ms, out, tc.want)
		}
	}
}
