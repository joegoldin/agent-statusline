package widgets

import (
	"strings"
	"testing"

	"github.com/joegoldin/agent-statusline/internal/voice"
)

func TestVoiceShowsModeWhenOn(t *testing.T) {
	w := &Voice{}
	ctx := &Context{VoiceProvider: func() *voice.Config { return &voice.Config{Enabled: true, Mode: "hold"} }}
	out, vis := w.Render(ctx)
	if !vis || !strings.Contains(out, "hold") {
		t.Errorf("got %q", out)
	}
}

func TestVoiceWithoutMode(t *testing.T) {
	w := &Voice{}
	ctx := &Context{VoiceProvider: func() *voice.Config { return &voice.Config{Enabled: true} }}
	out, vis := w.Render(ctx)
	if !vis {
		t.Fatal("expected visible")
	}
	if strings.Contains(out, "hold") || strings.Contains(out, "toggle") {
		t.Errorf("should not show a mode: %q", out)
	}
}

func TestVoiceHidesWhenOff(t *testing.T) {
	w := &Voice{}
	ctx := &Context{VoiceProvider: func() *voice.Config { return &voice.Config{Enabled: false} }}
	if _, vis := w.Render(ctx); vis {
		t.Errorf("expected hidden")
	}
}

func TestVoiceHidesWhenUnset(t *testing.T) {
	w := &Voice{}
	if _, vis := w.Render(&Context{VoiceProvider: func() *voice.Config { return nil }}); vis {
		t.Errorf("expected hidden")
	}
}
