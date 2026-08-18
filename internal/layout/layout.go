package layout

import (
	"strings"

	"github.com/joegoldin/agent-statusline/internal/render"
	"github.com/joegoldin/agent-statusline/internal/widgets"
)

// Separator joins segments within a row and (when merged) the two
// dashboard rows on a single line.
const Separator = " │ "

// separator is the package-internal alias kept for brevity in this file.
const separator = Separator

// FlexName is the reserved widget name that becomes a flex spacer.
const FlexName = "flex"

// flexMarker is a Widget that, when encountered in a row, indicates a
// flexible spacer position. It never produces visible content directly.
type flexMarker struct{}

func (flexMarker) Name() string                           { return FlexName }
func (flexMarker) Render(*widgets.Context) (string, bool) { return "", true }

// IsFlex returns true if w is a flex spacer marker.
func IsFlex(w widgets.Widget) bool {
	_, ok := w.(flexMarker)
	return ok
}

// FlexMarker returns a freshly-constructed flex marker.
func FlexMarker() widgets.Widget { return flexMarker{} }

// Options governs a single ComposeRow invocation.
type Options struct {
	Width        int
	DropPriority []string
	Hide         []string
}

type seg struct {
	name string
	text string
	flex bool
	drop int
}

// ComposeRow renders a single row from the given widgets and context.
func ComposeRow(row []widgets.Widget, _ []widgets.Widget, ctx *widgets.Context, opts Options) string {
	hide := stringSet(opts.Hide)
	dropMap := map[string]int{}
	for i, n := range opts.DropPriority {
		dropMap[n] = i
	}

	var segs []seg
	for _, w := range row {
		if hide[w.Name()] {
			continue
		}
		if IsFlex(w) {
			segs = append(segs, seg{name: FlexName, flex: true})
			continue
		}
		text, vis := widgets.SafeRender(w, ctx)
		if !vis || text == "" {
			continue
		}
		dp, ok := dropMap[w.Name()]
		if !ok {
			dp = len(opts.DropPriority) + 1
		}
		segs = append(segs, seg{name: w.Name(), text: text, drop: dp})
	}

	for {
		body := joinSegments(segs, opts.Width)
		if opts.Width <= 0 || render.VisibleWidth(body) <= opts.Width {
			return body
		}
		idx := lowestPriorityIdx(segs)
		if idx == -1 {
			return render.Truncate(body, opts.Width)
		}
		segs = append(segs[:idx], segs[idx+1:]...)
	}
}

// WrapRow packs a row's visible segments across as many lines as needed so
// nothing is dropped or truncated (except a lone segment wider than the
// terminal, which is truncated as a last resort). Flex spacers are ignored
// in wrap mode since right-alignment is meaningless once content wraps.
// Returns one rendered string per line.
func WrapRow(row []widgets.Widget, ctx *widgets.Context, opts Options) []string {
	hide := stringSet(opts.Hide)
	var texts []string
	for _, w := range row {
		if hide[w.Name()] || IsFlex(w) {
			continue
		}
		text, vis := widgets.SafeRender(w, ctx)
		if !vis || text == "" {
			continue
		}
		texts = append(texts, text)
	}
	return packLines(texts, opts.Width)
}

// packLines greedily fills lines with ` │ `-separated segments, breaking to
// a new line when the next segment wouldn't fit.
func packLines(texts []string, width int) []string {
	if len(texts) == 0 {
		return nil
	}
	sepW := render.VisibleWidth(separator)
	var lines []string
	cur := ""
	curW := 0
	for _, t := range texts {
		tw := render.VisibleWidth(t)
		switch {
		case cur == "":
			cur, curW = t, tw
		case width > 0 && curW+sepW+tw > width:
			lines = append(lines, cur)
			cur, curW = t, tw
		default:
			cur += separator + t
			curW += sepW + tw
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if width > 0 {
		for i := range lines {
			if render.VisibleWidth(lines[i]) > width {
				lines[i] = render.Truncate(lines[i], width)
			}
		}
	}
	return lines
}

// joinSegments emits segments with separators and expands flex spacers
// to fill remaining width.
func joinSegments(segs []seg, width int) string {
	fixed := 0
	flexCount := 0
	for _, s := range segs {
		if s.flex {
			flexCount++
			continue
		}
		fixed += render.VisibleWidth(s.text)
	}
	adjSeparators := 0
	for i := 1; i < len(segs); i++ {
		if !segs[i].flex && !segs[i-1].flex {
			adjSeparators++
		}
	}
	totalFixed := fixed + adjSeparators*render.VisibleWidth(separator)
	flexBudget := width - totalFixed
	if flexBudget < 0 {
		flexBudget = 0
	}
	perFlex := 0
	flexRemainder := 0
	if flexCount > 0 {
		perFlex = flexBudget / flexCount
		flexRemainder = flexBudget - perFlex*flexCount
	}

	var b strings.Builder
	prevWasVisible := false
	prevWasFlex := false
	for _, s := range segs {
		if s.flex {
			n := perFlex
			if flexRemainder > 0 {
				n++
				flexRemainder--
			}
			b.WriteString(strings.Repeat(" ", n))
			prevWasFlex = true
			prevWasVisible = true
			continue
		}
		if prevWasVisible && !prevWasFlex {
			b.WriteString(separator)
		}
		b.WriteString(s.text)
		prevWasVisible = true
		prevWasFlex = false
	}
	return b.String()
}

// lowestPriorityIdx returns the index of the dropable segment with the
// lowest drop priority, or -1 if no dropable segments remain.
func lowestPriorityIdx(segs []seg) int {
	best := -1
	for i, s := range segs {
		if s.flex {
			continue
		}
		if best == -1 || segs[i].drop < segs[best].drop {
			best = i
		}
	}
	return best
}

func stringSet(xs []string) map[string]bool {
	out := make(map[string]bool, len(xs))
	for _, x := range xs {
		out[x] = true
	}
	return out
}
