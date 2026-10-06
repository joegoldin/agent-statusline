package widgets

import (
	"reflect"
	"testing"

	"github.com/joegoldin/agent-statusline/internal/input"
	"github.com/joegoldin/agent-statusline/internal/render"
)

func TestParseLSPStrictness(t *testing.T) {
	tests := []struct {
		name string
		text string
		want lspState
		ok   bool
	}{
		{
			name: "one active server",
			text: "LSP Active: gopls",
			want: lspState{Up: true, Active: []string{"gopls"}},
			ok:   true,
		},
		{
			name: "several active servers",
			text: "LSP Active: typescript, rust-analyzer, nil",
			want: lspState{Up: true, Active: []string{"typescript", "rust-analyzer", "nil"}},
			ok:   true,
		},
		{
			name: "only failed servers",
			text: "LSP Failed: pyright",
			want: lspState{Down: true, Failed: []string{"pyright"}},
			ok:   true,
		},
		{
			name: "active and failed, themed as pi-lens publishes it",
			text: "\x1b[38;2;88;204;78mLSP Active: gopls, nil\x1b[39m · \x1b[38;2;224;71;71mLSP Failed: pyright\x1b[39m",
			want: lspState{Up: true, Down: true, Active: []string{"gopls", "nil"}, Failed: []string{"pyright"}},
			ok:   true,
		},
		{
			name: "compact, active",
			text: "LSP ✓",
			want: lspState{Up: true},
			ok:   true,
		},
		{
			name: "compact, active and failed",
			text: "LSP ✓ · LSP ✗",
			want: lspState{Up: true, Down: true},
			ok:   true,
		},
		{name: "inactive", text: "\x1b[2mLSP Inactive\x1b[22m"},
		// Compact failed and compact inactive are the same glyph in different
		// colours, so the uncoloured text cannot say which one it is.
		{name: "compact failed or inactive", text: "LSP ✗"},
		{name: "empty"},
		{name: "no ids", text: "LSP Active: "},
		{name: "failed before active", text: "LSP Failed: pyright · LSP Active: gopls"},
		{name: "a third part", text: "LSP Active: gopls · LSP Failed: pyright · LSP Idle: nil"},
		{name: "mixed compact and named", text: "LSP Active: gopls · LSP ✗"},
		{name: "a different format entirely", text: "lsp: 2 running"},
	}
	for _, tc := range tests {
		got, ok := parseLSP(tc.text)
		if ok != tc.ok {
			t.Errorf("%s: parseLSP(%q) ok = %v, want %v", tc.name, tc.text, ok, tc.ok)
			continue
		}
		if ok && !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: parseLSP(%q) = %+v, want %+v", tc.name, tc.text, got, tc.want)
		}
	}
}

func TestLSPRenderSpans(t *testing.T) {
	type span struct {
		intent render.Intent
		text   string
	}
	tests := []struct {
		name    string
		text    string
		width   int
		visible bool
		want    []span
	}{
		{
			name:    "active and failed",
			text:    "LSP Active: gopls, nil · LSP Failed: pyright",
			visible: true,
			want: []span{
				{render.IntentDanger, lspGlyph},
				{render.IntentOK, "✓ gopls, nil"},
				{render.IntentText, " "},
				{render.IntentDanger, "✗ pyright"},
			},
		},
		{
			name:    "active only",
			text:    "LSP Active: gopls",
			visible: true,
			want: []span{
				{render.IntentOK, lspGlyph},
				{render.IntentOK, "✓ gopls"},
			},
		},
		{
			name:    "failed only",
			text:    "LSP Failed: pyright",
			visible: true,
			want: []span{
				{render.IntentDanger, lspGlyph},
				{render.IntentDanger, "✗ pyright"},
			},
		},
		{
			name:    "compact terminal counts instead of naming",
			text:    "LSP Active: gopls, nil · LSP Failed: pyright",
			width:   40,
			visible: true,
			want: []span{
				{render.IntentDanger, lspGlyph},
				{render.IntentOK, "✓2"},
				{render.IntentText, " "},
				{render.IntentDanger, "✗1"},
			},
		},
		{
			name:    "pi-lens's compact form has nothing to count",
			text:    "LSP ✓ · LSP ✗",
			visible: true,
			want: []span{
				{render.IntentDanger, lspGlyph},
				{render.IntentOK, "✓"},
				{render.IntentText, " "},
				{render.IntentDanger, "✗"},
			},
		},
		{name: "inactive hides", text: "LSP Inactive"},
		{name: "unknown format hides", text: "LSP: warming up"},
		{name: "absent"},
	}
	for _, tc := range tests {
		ctx := &Context{Width: tc.width, Status: input.Status{LSP: tc.text}}
		spans, vis := LSP{}.RenderSpans(ctx)
		if vis != tc.visible {
			t.Errorf("%s: visible = %v, want %v (spans=%+v)", tc.name, vis, tc.visible, spans)
			continue
		}
		var got []span
		for _, s := range spans {
			got = append(got, span{s.Intent, s.Text})
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: spans = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}
