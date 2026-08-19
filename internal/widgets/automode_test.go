package widgets

import (
	"strings"
	"testing"

	"github.com/joegoldin/agent-statusline/internal/input"
)

func TestParseAutoModeStrictness(t *testing.T) {
	tests := []struct {
		name string
		text string
		want autoModeState
		ok   bool
	}{
		{
			name: "with the classifier pair",
			text: "AM● a:105 d:4 ca:89 cd:4",
			want: autoModeState{Enabled: true, Allowed: 105, Denied: 4, Classified: true, ClassifierAllowed: 89, ClassifierDenied: 4},
			ok:   true,
		},
		{
			name: "without the classifier pair",
			text: "AM● a:105 d:4",
			want: autoModeState{Enabled: true, Allowed: 105, Denied: 4},
			ok:   true,
		},
		{
			name: "disabled",
			text: "AM○ a:0 d:0",
			ok:   true,
		},
		{
			name: "themed by pi before it was republished",
			text: "\x1b[36mAM● a:7 d:1\x1b[39m",
			want: autoModeState{Enabled: true, Allowed: 7, Denied: 1},
			ok:   true,
		},
		{name: "empty"},
		{name: "missing the denied count", text: "AM● a:105"},
		{name: "half a classifier pair", text: "AM● a:105 d:4 ca:89"},
		{name: "unknown state glyph", text: "AM! a:1 d:2"},
		{name: "trailing junk", text: "AM● a:105 d:4 zz:9"},
		{name: "a different format entirely", text: "auto mode: on (105 allowed)"},
	}
	for _, tc := range tests {
		got, ok := parseAutoMode(tc.text)
		if ok != tc.ok {
			t.Errorf("%s: parseAutoMode(%q) ok = %v, want %v", tc.name, tc.text, ok, tc.ok)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("%s: parseAutoMode(%q) = %+v, want %+v", tc.name, tc.text, got, tc.want)
		}
	}
}

func TestAutoModeRender(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		visible bool
		want    []string
		absent  []string
	}{
		{
			name:    "full counts",
			text:    "AM● a:105 d:4 ca:89 cd:4",
			visible: true,
			want:    []string{"●", "105", "4", "89/4"},
		},
		{
			name:    "no classifier decisions yet",
			text:    "AM● a:12 d:0",
			visible: true,
			want:    []string{"●", "12", "0"},
			absent:  []string{"/"},
		},
		{
			name:    "disabled",
			text:    "AM○ a:0 d:0",
			visible: true,
			want:    []string{"○"},
		},
		{name: "unparsable hides rather than lying", text: "AM v2 105/4"},
		{name: "absent"},
	}
	for _, tc := range tests {
		ctx := &Context{Status: input.Status{AutoMode: tc.text}}
		out, vis := AutoMode{}.Render(ctx)
		if vis != tc.visible {
			t.Errorf("%s: visible = %v, want %v (out=%q)", tc.name, vis, tc.visible, out)
			continue
		}
		for _, want := range tc.want {
			if !strings.Contains(out, want) {
				t.Errorf("%s: %q missing from %q", tc.name, want, out)
			}
		}
		for _, absent := range tc.absent {
			if strings.Contains(out, absent) {
				t.Errorf("%s: %q should not appear in %q", tc.name, absent, out)
			}
		}
	}
}
