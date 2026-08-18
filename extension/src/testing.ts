import type { ThemeLike } from "./intents";

/**
 * A Theme double that records every colour request, so tests can assert which
 * theme tokens a renderer used rather than which bytes it produced. That is
 * the whole point of going native: assertions on token names survive a theme
 * change, assertions on escape codes do not.
 *
 * This file never ships — the Nix derivation deletes it alongside the tests,
 * because pi installs the extension with no node_modules beside it and this is
 * test-only scaffolding.
 */
export function recordingTheme(
  options: { colorMode?: "truecolor" | "256color"; ansi?: Record<string, string> } = {},
) {
  const seen: Array<[string, string]> = [];
  const ansi = options.ansi ?? {
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
  const theme: ThemeLike = {
    fg(color, text) {
      seen.push([color, text]);
      return `<${color}>${text}</${color}>`;
    },
    getFgAnsi(color) {
      seen.push([color, ""]);
      const v = ansi[color];
      if (!v) throw new Error(`Unknown theme color: ${color}`);
      return v;
    },
    bold(text) {
      return `<b>${text}</b>`;
    },
    getColorMode: () => options.colorMode ?? "truecolor",
  };
  return {
    theme,
    calls: () => seen,
    tokens: () => Array.from(new Set(seen.map(([c]) => c))),
  };
}
