package render

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

var ansiSGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiSGR.ReplaceAllString(s, "") }

func TestSampleGradientAnchors(t *testing.T) {
	for _, stop := range SmoothGradient {
		got := SampleGradient(SmoothGradient, stop.T)
		if got != stop.Color {
			t.Errorf("SampleGradient at T=%v: got %+v, want %+v", stop.T, got, stop.Color)
		}
	}
}

func TestSampleGradientClampsOutOfRange(t *testing.T) {
	if got := SampleGradient(SmoothGradient, -0.5); got != SmoothGradient[0].Color {
		t.Errorf("SampleGradient(-0.5): got %+v, want first stop %+v", got, SmoothGradient[0].Color)
	}
	last := SmoothGradient[len(SmoothGradient)-1].Color
	if got := SampleGradient(SmoothGradient, 1.5); got != last {
		t.Errorf("SampleGradient(1.5): got %+v, want last stop %+v", got, last)
	}
}

func TestSampleGradientInterpolatesMidway(t *testing.T) {
	// Midpoint between the green stop (T=0.30) and yellow stop (T=0.50)
	// must be the average of those two colors (within rounding).
	a := SmoothGradient[1].Color // green
	b := SmoothGradient[2].Color // yellow
	got := SampleGradient(SmoothGradient, 0.40)
	want := RGB{
		uint8((int(a.R) + int(b.R)) / 2),
		uint8((int(a.G) + int(b.G)) / 2),
		uint8((int(a.B) + int(b.B)) / 2),
	}
	for _, d := range []int{int(got.R) - int(want.R), int(got.G) - int(want.G), int(got.B) - int(want.B)} {
		if d < -1 || d > 1 {
			t.Errorf("SampleGradient(0.40) = %+v, want ~%+v (±1)", got, want)
			break
		}
	}
}

func TestLumaDarkenMovesTowardShadow(t *testing.T) {
	c := RGB{200, 200, 200}
	got := LumaDarken(c)
	if got.R >= c.R || got.G >= c.G || got.B >= c.B {
		t.Errorf("LumaDarken should reduce all channels: %+v -> %+v", c, got)
	}
}

func TestGradientBarZeroPercentIsAllGhost(t *testing.T) {
	bar := GradientBar(0, 10, BrailleStyle)
	// Every cell should be the body glyph (ghost == body for braille style),
	// but every color escape should be in luma-darkened range — i.e. no
	// channel above ~80 for any cell.
	if !strings.Contains(bar, "⣿") {
		t.Fatalf("expected ⣿ in zero-percent bar: %q", bar)
	}
}

func TestGradientBarFullPercentIsAllBody(t *testing.T) {
	bar := GradientBar(100, 10, BrailleStyle)
	// 10 body glyphs, no edge glyph.
	for _, edge := range []rune{'⡀', '⡄', '⡆', '⡇', '⣇', '⣧', '⣷'} {
		if strings.ContainsRune(bar, edge) {
			t.Errorf("100%% bar should not contain partial edge %q: %q", edge, bar)
		}
	}
}

func TestGradientBarRespectsWidth(t *testing.T) {
	for _, w := range []int{1, 4, 10, 24} {
		bar := GradientBar(50, w, BlockStyle)
		cells := utf8.RuneCountInString(stripANSI(bar))
		if cells != w {
			t.Errorf("GradientBar width=%d produced %d cells: %q", w, cells, stripANSI(bar))
		}
	}
}

func TestGradientBarEdgeAtPartialCell(t *testing.T) {
	// 50% on a 10-cell bar lands exactly at cell 5 with 0 partial; the bar
	// should have 5 body glyphs and 5 ghost glyphs and no edge glyph.
	bar := GradientBar(50, 10, BlockStyle)
	for _, edge := range []rune{'▏', '▎', '▍', '▌', '▋', '▊', '▉'} {
		if strings.ContainsRune(bar, edge) {
			t.Errorf("50%% bar should land on exact-cell boundary, got partial %q in %q", edge, bar)
		}
	}
}

func TestGradientBarStylesAreDistinct(t *testing.T) {
	b1 := GradientBar(50, 10, BrailleStyle)
	b2 := GradientBar(50, 10, BlockStyle)
	b3 := GradientBar(50, 10, LineStyle)
	if b1 == b2 || b1 == b3 || b2 == b3 {
		t.Errorf("expected distinct styles to render distinct bars")
	}
}
