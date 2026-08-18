package input

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Mode selects which harness produced the stdin payload.
type Mode string

const (
	ModeClaude Mode = "claude"
	ModePi     Mode = "pi"
	// ModeAuto defers the choice to Detect.
	ModeAuto Mode = "auto"
)

// ParseMode converts a --mode flag value into a Mode. The empty string means
// auto, so an unset flag behaves like the documented default.
func ParseMode(s string) (Mode, error) {
	switch Mode(s) {
	case "", ModeAuto:
		return ModeAuto, nil
	case ModeClaude:
		return ModeClaude, nil
	case ModePi:
		return ModePi, nil
	}
	return "", fmt.Errorf("unknown mode %q (want claude, pi, or auto)", s)
}

// discriminator is the single field pi payloads set, so Detect never has to
// guess from the shape of optional fields.
type discriminator struct {
	Harness string `json:"harness"`
}

// Detect infers the Mode from a raw payload. Claude Code's payload carries no
// discriminator, so anything not explicitly pi is treated as Claude —
// including malformed input, which then fails in DecodeClaude with a useful
// error rather than silently here.
func Detect(raw []byte) Mode {
	var d discriminator
	if err := json.Unmarshal(raw, &d); err != nil {
		return ModeClaude
	}
	if Mode(d.Harness) == ModePi {
		return ModePi
	}
	return ModeClaude
}

// Decode reads r fully and decodes it according to m. ModeAuto detects first.
func Decode(r io.Reader, m Mode) (Status, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return Status{}, err
	}
	if m == ModeAuto {
		m = Detect(raw)
	}
	switch m {
	case ModePi:
		return DecodePi(bytes.NewReader(raw))
	default:
		return DecodeClaude(bytes.NewReader(raw))
	}
}
