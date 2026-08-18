package render

import (
	"fmt"
	"strings"
)

// RGB is an 8-bit-per-channel truecolor triplet.
type RGB struct{ R, G, B uint8 }

// TrueColor wraps s in a 24-bit foreground SGR escape and a reset.
func TrueColor(c RGB, s string) string {
	if s == "" {
		return ""
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s%s", c.R, c.G, c.B, s, reset)
}

// ColorStop anchors a smooth gradient at position T in [0,1]; values between
// stops are linearly interpolated in RGB space.
type ColorStop struct {
	T     float64
	Color RGB
}

// SmoothGradient is the per-cell rainbow shared by every bar — dim → green
// → yellow → orange → red. Colors approximate the semantic ANSI palette so
// the bar's leading-edge hue agrees with the surrounding ThresholdColor*
// text at zone boundaries.
var SmoothGradient = []ColorStop{
	{0.00, RGB{130, 135, 140}},
	{0.30, RGB{88, 204, 78}},
	{0.50, RGB{236, 200, 64}},
	{0.65, RGB{255, 135, 0}},
	{0.85, RGB{224, 71, 71}},
	{1.00, RGB{224, 71, 71}},
}

// shadow is the target color the ghost track is mixed toward — near-black
// with a faint violet bias so unfilled cells read as "off but present".
var shadow = RGB{22, 18, 28}

const shadowMix = 0.83

func lerp8(a, b uint8, t float64) uint8 {
	v := float64(a) + (float64(b)-float64(a))*t + 0.5
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func lerpRGB(a, b RGB, t float64) RGB {
	return RGB{lerp8(a.R, b.R, t), lerp8(a.G, b.G, t), lerp8(a.B, b.B, t)}
}

// SampleGradient returns the interpolated color at t in [0,1]. Stops must be
// sorted by T ascending; the function clamps to the first/last stop for
// out-of-range t.
func SampleGradient(stops []ColorStop, t float64) RGB {
	if len(stops) == 0 {
		return RGB{}
	}
	if t <= stops[0].T {
		return stops[0].Color
	}
	if t >= stops[len(stops)-1].T {
		return stops[len(stops)-1].Color
	}
	for i := 0; i < len(stops)-1; i++ {
		a, b := stops[i], stops[i+1]
		if t <= b.T {
			local := (t - a.T) / (b.T - a.T)
			return lerpRGB(a.Color, b.Color, local)
		}
	}
	return stops[len(stops)-1].Color
}

// LumaDarken mixes c toward the shadow target, producing the ghost-track
// color used for unfilled cells.
func LumaDarken(c RGB) RGB {
	return lerpRGB(c, shadow, shadowMix)
}

// BarStyle bundles the glyphs used to draw a gradient bar. Body fills lit
// cells, Ghost fills unfilled "track" cells (LumaDarken'd), and Edge supplies
// a per-cell sub-pixel partial at the leading edge. Edge[0] is the empty
// sentinel and unused; Edge[1..len-1] are monotonically larger partial fills.
type BarStyle struct {
	Body  rune
	Ghost rune
	Edge  []rune
}

// BrailleStyle — focal bar (highest density, 8-step vertical-fill edge).
// Used by the context window where the user looks most often.
var BrailleStyle = BarStyle{
	Body:  '⣿',
	Ghost: '⣿',
	Edge:  []rune("⠀⡀⡄⡆⡇⣇⣧⣷⣿"),
}

// BlockStyle — solid bar (full ▉-weight, 8-step horizontal sub-pixel edge).
// Used by the actively-ticking 5h rate-limit window.
var BlockStyle = BarStyle{
	Body:  '█',
	Ghost: '█',
	Edge:  []rune(" ▏▎▍▌▋▊▉█"),
}

// LineStyle — slim wire (single horizontal stroke, binary edge). Used by
// the 7d budget bar where fractional fill isn't perceptible at that pace.
var LineStyle = BarStyle{
	Body:  '━',
	Ghost: '━',
	Edge:  []rune(" ━"),
}

// GradientBar renders a width-cell bar at pct (0–100). Each cell carries its
// own color: filled cells get SmoothGradient at the cell's position; unfilled
// cells get the same color LumaDarken'd. The leading edge uses style.Edge
// for sub-pixel partial fill.
func GradientBar(pct float64, width int, style BarStyle) string {
	if width <= 0 {
		return ""
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	edgeSteps := len(style.Edge) - 1
	if edgeSteps < 1 {
		edgeSteps = 1
	}
	fill := pct / 100 * float64(width)
	full := int(fill)
	partial := fill - float64(full)

	var b strings.Builder
	for i := 0; i < width; i++ {
		t := (float64(i) + 0.5) / float64(width)
		col := SampleGradient(SmoothGradient, t)
		switch {
		case i < full:
			b.WriteString(TrueColor(col, string(style.Body)))
		case i == full && partial > 0:
			idx := int(partial*float64(edgeSteps) + 0.5)
			if idx < 1 {
				idx = 1
			}
			if idx > edgeSteps {
				idx = edgeSteps
			}
			b.WriteString(TrueColor(col, string(style.Edge[idx])))
		default:
			b.WriteString(TrueColor(LumaDarken(col), string(style.Ghost)))
		}
	}
	return b.String()
}
