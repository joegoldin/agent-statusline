import type { ThemeLike } from "./intents";

// Two palettes, because a theme's colour mode changes the bytes it emits and
// the bar renderer branches on exactly that. Shapes copied from pi's
// fgAnsi(value, mode) (theme.ts): 38;2;r;g;b on truecolor, 38;5;N on 256.
const TRUECOLOR: Record<string, string> = {
  text: "\x1b[38;2;220;220;220m",
  dim: "\x1b[38;2;130;135;140m",
  muted: "\x1b[38;2;150;150;150m",
  accent: "\x1b[38;2;0;175;215m",
  success: "\x1b[38;2;88;204;78m",
  warning: "\x1b[38;2;236;200;64m",
  error: "\x1b[38;2;224;71;71m",
  mdLink: "\x1b[38;2;95;175;255m",
  customMessageLabel: "\x1b[38;2;200;120;255m",
};

const INDEXED: Record<string, string> = {
  text: "\x1b[38;5;252m",
  dim: "\x1b[38;5;245m",
  muted: "\x1b[38;5;244m",
  accent: "\x1b[38;5;38m",
  success: "\x1b[38;5;77m",
  warning: "\x1b[38;5;185m",
  error: "\x1b[38;5;167m",
  mdLink: "\x1b[38;5;75m",
  customMessageLabel: "\x1b[38;5;177m",
};

/**
 * A Theme double that records every colour request, so tests can assert which
 * theme tokens a renderer used rather than which bytes it produced. That is
 * the whole point of going native: assertions on token names survive a theme
 * change, assertions on escape codes do not.
 *
 * It emits the same *shape* of output as the real Theme — `fg` wraps in the
 * colour's SGR prefix and closes with the foreground reset, `bold` uses SGR 1
 * — because half the assertions in this suite are about how wide a rendered
 * line is. A double that wrapped text in readable markers would count those
 * markers as visible cells and every width assertion would measure the double
 * instead of the renderer.
 *
 * This file never ships: the Nix derivation deletes it alongside the tests,
 * since pi installs the extension with no node_modules beside it.
 */
export function recordingTheme(
  options: { colorMode?: "truecolor" | "256color"; ansi?: Record<string, string> } = {},
) {
  const seen: Array<[string, string]> = [];
  const mode = options.colorMode ?? "truecolor";
  const ansi = options.ansi ?? (mode === "256color" ? INDEXED : TRUECOLOR);
  const lookup = (color: string): string => {
    const v = ansi[color];
    // pi's Theme.fg and getFgAnsi both throw on an unknown slot (theme.ts:390,
    // :422). A double that returned "" instead would hide every unguarded call.
    if (!v) throw new Error(`Unknown theme color: ${color}`);
    return v;
  };
  const theme: ThemeLike = {
    fg(color, text) {
      seen.push([color, text]);
      return `${lookup(color)}${text}\x1b[39m`;
    },
    getFgAnsi(color) {
      seen.push([color, ""]);
      return lookup(color);
    },
    bold(text) {
      return `\x1b[1m${text}\x1b[22m`;
    },
    getColorMode: () => mode,
  };
  return {
    theme,
    calls: () => seen,
    tokens: () => Array.from(new Set(seen.map(([c]) => c))),
  };
}
