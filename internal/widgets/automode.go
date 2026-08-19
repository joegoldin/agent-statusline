package widgets

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/joegoldin/agent-statusline/internal/render"
)

const (
	autoGlyph       = " " // nf-fa-magic
	classifierGlyph = " " // nf-fa-balance_scale
)

// autoModeState is one parse of the pi-automode status text.
type autoModeState struct {
	Enabled           bool
	Allowed           int
	Denied            int
	Classified        bool
	ClassifierAllowed int
	ClassifierDenied  int
}

// autoModeRE matches the whole of auto mode's status text and nothing else:
// "AM● a:105 d:4" with an optional " ca:89 cd:4" tail, exactly as
// extensions/auto-mode/state.ts builds it. Anchored and total on purpose. The
// counts are the only thing this widget knows about auto mode, so an upstream
// format change has to degrade to a missing widget rather than to numbers
// scraped out of a line that now means something else.
var autoModeRE = regexp.MustCompile(`^AM([●○]) a:(\d+) d:(\d+)(?: ca:(\d+) cd:(\d+))?$`)

// ansiRE strips the SGR wrapper pi's theme puts around the text before auto
// mode hands it to setStatus, which is what the republishing shim captures.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func parseAutoMode(text string) (autoModeState, bool) {
	m := autoModeRE.FindStringSubmatch(strings.TrimSpace(ansiRE.ReplaceAllString(text, "")))
	if m == nil {
		return autoModeState{}, false
	}
	// Every capture is \d+ or the whole optional group, so the conversions
	// cannot fail and an empty group is a legitimate zero.
	atoi := func(s string) int { n, _ := strconv.Atoi(s); return n }
	return autoModeState{
		Enabled:           m[1] == "●",
		Allowed:           atoi(m[2]),
		Denied:            atoi(m[3]),
		Classified:        m[4] != "",
		ClassifierAllowed: atoi(m[4]),
		ClassifierDenied:  atoi(m[5]),
	}, true
}

// AutoMode renders the pi-automode extension's permission tally: whether auto
// mode is on, how many actions it allowed and blocked, and how the classifier
// split the calls it was asked about.
type AutoMode struct{}

func (AutoMode) Name() string { return "autoMode" }

func (AutoMode) Render(ctx *Context) (string, bool) {
	spans, ok := AutoMode{}.RenderSpans(ctx)
	return spans.ANSI(), ok
}

func (AutoMode) RenderSpans(ctx *Context) (render.Spans, bool) {
	s, ok := parseAutoMode(ctx.Status.AutoMode)
	if !ok {
		return nil, false
	}
	stateIntent := render.IntentDim
	circle := "○"
	if s.Enabled {
		stateIntent = render.IntentOK
		circle = "●"
	}
	label := autoGlyph + "auto "
	if ctx.Compact() {
		label = autoGlyph
	}
	deniedIntent := render.IntentDim
	if s.Denied > 0 {
		deniedIntent = render.IntentDanger
	}
	spans := render.Spans{
		render.Text(stateIntent, label+circle),
		render.Text(render.IntentText, rowSeparator),
		render.Text(render.IntentOK, "✓ "+strconv.Itoa(s.Allowed)),
		render.Text(render.IntentText, " "),
		render.Text(deniedIntent, "✗ "+strconv.Itoa(s.Denied)),
	}
	// The classifier pair is absent from the source text until the classifier
	// has actually decided something, and dropping it in compact mode keeps the
	// row to the tally that matters on a narrow terminal.
	if !s.Classified || ctx.Compact() {
		return spans, true
	}
	return append(spans,
		render.Text(render.IntentText, rowSeparator),
		render.Text(render.IntentMeta, classifierGlyph+strconv.Itoa(s.ClassifierAllowed)+"/"+strconv.Itoa(s.ClassifierDenied)),
	), true
}
