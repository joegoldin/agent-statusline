package widgets

import (
	"strings"
	"testing"

	"github.com/joegoldin/agent-statusline/internal/config"
	"github.com/joegoldin/agent-statusline/internal/input"
)

func TestTokensCompact(t *testing.T) {
	w := &Tokens{}
	ctx := &Context{
		Status: input.Status{ContextWindow: &input.ContextWindow{TotalInputTokens: 444139}},
		Cfg:    config.Config{TokenFormat: "compact"},
	}
	out, vis := w.Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	if !strings.Contains(out, "444.1k") {
		t.Errorf("expected 444.1k in %q", out)
	}
	if !strings.Contains(out, "tokens") {
		t.Errorf("expected 'tokens' suffix in %q", out)
	}
}

func TestTokensRaw(t *testing.T) {
	w := &Tokens{}
	ctx := &Context{
		Status: input.Status{ContextWindow: &input.ContextWindow{TotalInputTokens: 444139}},
		Cfg:    config.Config{TokenFormat: "raw"},
	}
	out, _ := w.Render(ctx)
	if !strings.Contains(out, "444139") {
		t.Errorf("expected raw count in %q", out)
	}
	if !strings.Contains(out, "tokens") {
		t.Errorf("expected 'tokens' suffix in %q", out)
	}
}

func TestTokensFallsBackToCurrentUsage(t *testing.T) {
	w := &Tokens{}
	ctx := &Context{
		Status: input.Status{ContextWindow: &input.ContextWindow{
			CurrentUsage: &input.CurrentUsage{
				InputTokens:              10000,
				CacheCreationInputTokens: 5000,
				CacheReadInputTokens:     20000,
			},
		}},
		Cfg: config.Config{TokenFormat: "compact"},
	}
	out, vis := w.Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	if !strings.Contains(out, "35.0k") {
		t.Errorf("expected 35.0k in %q", out)
	}
}

func TestTokensHidesWhenAbsent(t *testing.T) {
	if _, vis := (&Tokens{}).Render(&Context{}); vis {
		t.Errorf("expected hidden")
	}
}
