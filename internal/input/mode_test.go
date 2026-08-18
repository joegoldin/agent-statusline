package input

import (
	"strings"
	"testing"
)

func TestParseMode(t *testing.T) {
	cases := []struct {
		in      string
		want    Mode
		wantErr bool
	}{
		{"claude", ModeClaude, false},
		{"pi", ModePi, false},
		{"auto", ModeAuto, false},
		{"", ModeAuto, false},
		{"bogus", "", true},
	}
	for _, c := range cases {
		got, err := ParseMode(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseMode(%q): want error, got nil", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseMode(%q): unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseMode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDetect(t *testing.T) {
	if got := Detect([]byte(`{"harness":"pi","cwd":"/x"}`)); got != ModePi {
		t.Errorf("Detect(pi payload) = %q, want %q", got, ModePi)
	}
	if got := Detect([]byte(`{"cwd":"/x","session_id":"a"}`)); got != ModeClaude {
		t.Errorf("Detect(claude payload) = %q, want %q", got, ModeClaude)
	}
	if got := Detect([]byte(`not json`)); got != ModeClaude {
		t.Errorf("Detect(garbage) = %q, want %q", got, ModeClaude)
	}
}

func TestDecodeAutoSelectsClaude(t *testing.T) {
	s, err := Decode(strings.NewReader(sampleJSON), ModeAuto)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if s.SessionID != "abc-123" {
		t.Errorf("SessionID = %q, want %q", s.SessionID, "abc-123")
	}
}
