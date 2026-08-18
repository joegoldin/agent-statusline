package widgets

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const cwdGlyph = " " // nf-fa-folder

type CWD struct{}

func (CWD) Name() string { return "cwd" }

func (CWD) Render(ctx *Context) (string, bool) {
	spans, ok := CWD{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (CWD) RenderSpans(ctx *Context) (render.Spans, bool) {
	path := ctx.Status.CWD
	if path == "" {
		path = ctx.Status.Workspace.CurrentDir
	}
	if path == "" {
		return nil, false
	}
	home, _ := os.UserHomeDir()
	prefix := ""
	if home != "" && strings.HasPrefix(path, home) {
		prefix = "~"
		path = strings.TrimPrefix(path, home)
	}
	path = lastNSegments(path, 2)
	// IntentPath, not IntentWarn: the same SGR 33 under Claude Code, but pi
	// renders a directory as a link rather than as an alarm.
	return render.Spans{render.Text(render.IntentPath, cwdGlyph+prefix+path)}, true
}

// lastNSegments keeps only the last n segments of p (joined by separator),
// prepending ".../" when truncation happened.
func lastNSegments(p string, n int) string {
	if n <= 0 || p == "" {
		return p
	}
	sep := string(filepath.Separator)
	parts := strings.Split(strings.TrimPrefix(p, sep), sep)
	if len(parts) <= n {
		return p
	}
	tail := parts[len(parts)-n:]
	return "/.../" + strings.Join(tail, sep)
}
