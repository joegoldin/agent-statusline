package widgets

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const lspGlyph = "\uf121 " // nf-fa-code

// lspState is one parse of pi-lens's "pi-lens-lsp" status text.
type lspState struct {
	// Up and Down say whether any server is running or has failed. They are
	// separate from the id lists because pi-lens's compact form publishes the
	// state without naming the servers.
	Up, Down bool
	// Active and Failed are the server ids, nil in the compact form.
	Active, Failed []string
}

// lspActiveRE and lspFailedRE each match one whole " · "-separated part of
// pi-lens's status text, exactly as updateLspStatus in its dist builds it:
// `LSP Active: ${ids.join(", ")}` and `LSP Failed: ${ids.join(", ")}`.
// Anchored and total on purpose, for the same reason as autoModeRE: a format
// change has to cost the widget, not put a stray word in the server list.
var (
	lspActiveRE = regexp.MustCompile(`^LSP Active: ([^\s,·]+(?:, [^\s,·]+)*)$`)
	lspFailedRE = regexp.MustCompile(`^LSP Failed: ([^\s,·]+(?:, [^\s,·]+)*)$`)
)

const (
	lspPartSeparator = " · "
	lspCompactUp     = "LSP ✓"
	lspCompactDown   = "LSP ✗"
)

// parseLSP reads the status text after stripping the theme's SGR wrappers.
// "LSP Inactive" is reported as not ok: nothing is running and nothing is
// wrong, so there is nothing worth a slot. So is a bare "LSP ✗": pi-lens's
// compact form uses it both for a failed server (in red) and for no server at
// all (dimmed), and with the colour stripped the two cannot be told apart.
func parseLSP(text string) (lspState, bool) {
	plain := strings.TrimSpace(ansiRE.ReplaceAllString(text, ""))
	if plain == "" {
		return lspState{}, false
	}
	parts := strings.Split(plain, lspPartSeparator)
	switch {
	case len(parts) == 1 && plain == lspCompactUp:
		return lspState{Up: true}, true
	case len(parts) == 2 && parts[0] == lspCompactUp && parts[1] == lspCompactDown:
		return lspState{Up: true, Down: true}, true
	}

	var s lspState
	rest := parts
	if m := lspActiveRE.FindStringSubmatch(rest[0]); m != nil {
		s.Up, s.Active = true, strings.Split(m[1], ", ")
		rest = rest[1:]
	}
	if len(rest) > 0 {
		m := lspFailedRE.FindStringSubmatch(rest[0])
		if m == nil {
			return lspState{}, false
		}
		s.Down, s.Failed = true, strings.Split(m[1], ", ")
		rest = rest[1:]
	}
	// pi-lens publishes at most an active part followed by a failed part;
	// anything after them is a format this parser has not seen.
	if len(rest) > 0 || (!s.Up && !s.Down) {
		return lspState{}, false
	}
	return s, true
}

// LSP renders pi-lens's language-server status: the servers that are up in
// the OK intent, the ones that failed in the danger intent.
type LSP struct{}

func (LSP) Name() string { return "lsp" }

func (LSP) Render(ctx *Context) (string, bool) {
	spans, ok := LSP{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (LSP) RenderSpans(ctx *Context) (render.Spans, bool) {
	s, ok := parseLSP(ctx.Status.LSP)
	if !ok {
		return nil, false
	}
	// The glyph takes the worst state, so a failure is visible even when the
	// row is too narrow to say which server it was.
	glyphIntent := render.IntentOK
	if s.Down {
		glyphIntent = render.IntentDanger
	}
	spans := render.Spans{render.Text(glyphIntent, lspGlyph)}
	if s.Up {
		spans = append(spans, render.Text(render.IntentOK, lspFigure("✓", s.Active, ctx.Compact())))
	}
	if s.Down {
		if s.Up {
			spans = append(spans, render.Text(render.IntentText, " "))
		}
		spans = append(spans, render.Text(render.IntentDanger, lspFigure("✗", s.Failed, ctx.Compact())))
	}
	return spans, true
}

// lspFigure names the servers in a state, or counts them in compact mode.
// Without ids (pi-lens's own compact form) it is the mark alone either way.
func lspFigure(mark string, ids []string, compact bool) string {
	switch {
	case len(ids) == 0:
		return mark
	case compact:
		return mark + strconv.Itoa(len(ids))
	default:
		return mark + " " + strings.Join(ids, ", ")
	}
}
