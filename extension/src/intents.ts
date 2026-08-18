import { renderBar } from "./bars";
import type { Intent, SnapshotConfig, Span } from "./snapshot";

/**
 * The slice of pi's Theme this renderer uses. Declared structurally rather
 * than imported so the runtime file has no dependency; pi hands the real Theme
 * to the component factory and it satisfies this shape.
 * See pi's src/modes/interactive/theme/theme.ts:390-437 — fg() and getFgAnsi()
 * both throw on an unknown slot, which is why every call here is guarded.
 */
export interface ThemeLike {
  fg(color: string, text: string): string;
  getFgAnsi(color: string): string;
  bold(text: string): string;
  getColorMode(): "truecolor" | "256color";
}

/**
 * Semantic intent to pi theme token. This is the other half of the table in
 * internal/render/span.go, and the reason the statusline finally follows
 * /theme instead of hardcoding SGR 31..36.
 *
 * Two entries are worth reading twice:
 *
 *  - `path` maps to mdLink while `warn` maps to warning. Under Claude Code
 *    both emit SGR 33, because that palette has no separate colour for a
 *    directory. A theme does, so here they part company.
 *  - `caution` is the 4th step of the five-step context ramp, which is orange
 *    in the Claude palette. pi themes expose no orange slot (ThemeColor,
 *    theme.ts:112-159), so it becomes a bolded warning: still visibly hotter
 *    than warn, still a colour the user chose.
 */
export const INTENT_TOKENS: Record<Intent, { token: string; bold?: boolean }> = {
  text: { token: "text" },
  dim: { token: "dim" },
  muted: { token: "muted" },
  accent: { token: "accent" },
  meta: { token: "customMessageLabel" },
  path: { token: "mdLink" },
  ok: { token: "success" },
  warn: { token: "warning" },
  caution: { token: "warning", bold: true },
  danger: { token: "error" },
};

function colourise(text: string, intent: Intent | undefined, theme: ThemeLike): string {
  if (text === "") return "";
  const mapping = INTENT_TOKENS[intent ?? "text"] ?? INTENT_TOKENS.text;
  let out: string;
  try {
    out = theme.fg(mapping.token, text);
  } catch {
    // An older or hand-written theme may lack an optional slot; falling back
    // to the default foreground loses a colour, never a widget.
    try {
      out = theme.fg("text", text);
    } catch {
      out = text;
    }
  }
  return mapping.bold ? theme.bold(out) : out;
}

/** OSC 8 hyperlink, applied outside the colour so the link covers the run. */
function hyperlink(url: string, text: string): string {
  return `\x1b]8;;${url}\x1b\\${text}\x1b]8;;\x1b\\`;
}

/** Render one widget's spans into a coloured string. */
export function paint(spans: Span[], theme: ThemeLike, cfg: SnapshotConfig): string {
  let out = "";
  for (const span of spans) {
    if (span.kind === "bar") {
      out += renderBar(span.fill ?? 0, span.cells ?? cfg.barWidth, span.style ?? "block", theme);
      continue;
    }
    const coloured = colourise(span.text ?? "", span.intent, theme);
    out += span.link && coloured ? hyperlink(span.link, coloured) : coloured;
  }
  return out;
}
