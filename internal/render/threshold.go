package render

// ThresholdColor returns the colorizer for a 0–100 percentage: green <70,
// yellow 70–85, red >=85. Used for the surrounding text (glyph + percent)
// next to a bar so the alarm signal is sharp even when the bar itself
// renders a smooth gradient.
func ThresholdColor(pct float64) func(string) string {
	switch {
	case pct >= 85:
		return Red
	case pct >= 70:
		return Yellow
	default:
		return Green
	}
}

// ThresholdColor5 returns the colorizer for a 0–100 percentage as a
// five-step gradient for finer-grained signaling on non-bar widgets:
//
//	<30   dim (gray)  — barely used
//	<45   green       — comfortable
//	<60   yellow      — getting full
//	<75   orange      — warning
//	>=75  red         — critical
//
// Thresholds bias warmer earlier than the 3-step palette so context
// pressure shows up well before the conversation actually hits a wall.
func ThresholdColor5(pct float64) func(string) string {
	switch {
	case pct >= 75:
		return Red
	case pct >= 60:
		return Orange
	case pct >= 45:
		return Yellow
	case pct >= 30:
		return Green
	default:
		return Dim
	}
}
