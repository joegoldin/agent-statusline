import type { ThemeLike } from "./intents";

/**
 * Glyph sets, mirroring internal/render/gradient.go rune for rune. Edge[0] is
 * the empty sentinel; Edge[1..] are monotonically larger partial fills, giving
 * the leading cell sub-cell resolution.
 */
const STYLES: Record<string, { body: string; ghost: string; edge: string[] }> = {
  braille: { body: "⣿", ghost: "⣿", edge: Array.from("⠀⡀⡄⡆⡇⣇⣧⣷⣿") },
  block: { body: "█", ghost: "█", edge: Array.from(" ▏▎▍▌▋▊▉█") },
  line: { body: "━", ghost: "━", edge: Array.from(" ━") },
};

/**
 * The ramp anchors, as theme tokens rather than RGB. These are the semantic
 * equivalent of SmoothGradient's stops in internal/render/gradient.go, which
 * hardcodes grey to green to yellow to orange to red. The orange stop has no
 * theme slot, so the pi ramp has four anchors rather than five — the one
 * fidelity loss of going native, and a deliberate one.
 */
const RAMP: ReadonlyArray<readonly [number, string]> = [
  [0.0, "dim"],
  [0.3, "success"],
  [0.5, "warning"],
  [1.0, "error"],
];

interface RGB {
  r: number;
  g: number;
  b: number;
}

const CUBE = [0, 95, 135, 175, 215, 255];

/**
 * Recover RGB from a theme's foreground SGR prefix. pi builds these with
 * fgAnsi(value, mode) (theme.ts), which emits 38;2;r;g;b on truecolor themes
 * and 38;5;N on 256-colour ones, so both forms are parseable. Returns
 * undefined for anything else, which pushes the caller onto the discrete path.
 */
export function parseFgAnsi(ansi: string): RGB | undefined {
  const truecolor = /38;2;(\d+);(\d+);(\d+)/.exec(ansi);
  if (truecolor) {
    return { r: +truecolor[1]!, g: +truecolor[2]!, b: +truecolor[3]! };
  }
  const indexed = /38;5;(\d+)/.exec(ansi);
  if (!indexed) return undefined;
  const n = +indexed[1]!;
  if (n >= 232) {
    const v = 8 + (n - 232) * 10;
    return { r: v, g: v, b: v };
  }
  if (n >= 16) {
    const i = n - 16;
    return { r: CUBE[Math.floor(i / 36)]!, g: CUBE[Math.floor(i / 6) % 6]!, b: CUBE[i % 6]! };
  }
  const base = n % 8;
  const bright = n >= 8 ? 255 : 170;
  return { r: base & 1 ? bright : 0, g: base & 2 ? bright : 0, b: base & 4 ? bright : 0 };
}

function lerp(a: number, b: number, t: number): number {
  return Math.max(0, Math.min(255, Math.round(a + (b - a) * t)));
}

// The ghost track: unfilled cells mixed toward near-black with a faint violet
// bias, the same shadow target and mix factor as internal/render/gradient.go.
const SHADOW: RGB = { r: 22, g: 18, b: 28 };
const SHADOW_MIX = 0.83;

function darken(c: RGB): RGB {
  return {
    r: lerp(c.r, SHADOW.r, SHADOW_MIX),
    g: lerp(c.g, SHADOW.g, SHADOW_MIX),
    b: lerp(c.b, SHADOW.b, SHADOW_MIX),
  };
}

function tokenAt(t: number): string {
  let token = RAMP[0]![1];
  for (const [stop, name] of RAMP) {
    if (t >= stop) token = name;
  }
  return token;
}

function sampleRamp(t: number, anchors: Array<[number, RGB]>): RGB {
  if (t <= anchors[0]![0]) return anchors[0]![1];
  const last = anchors[anchors.length - 1]!;
  if (t >= last[0]) return last[1];
  for (let i = 0; i < anchors.length - 1; i++) {
    const [ta, ca] = anchors[i]!;
    const [tb, cb] = anchors[i + 1]!;
    if (t <= tb) {
      const local = (t - ta) / (tb - ta);
      return { r: lerp(ca.r, cb.r, local), g: lerp(ca.g, cb.g, local), b: lerp(ca.b, cb.b, local) };
    }
  }
  return last[1];
}

function truecolorCell(c: RGB, glyph: string): string {
  return `\x1b[38;2;${c.r};${c.g};${c.b}m${glyph}\x1b[39m`;
}

/**
 * Render a width-`cells` bar at `fill` (a fraction in [0,1]).
 *
 * On a truecolor theme the ramp anchors are parsed back out of
 * theme.getFgAnsi and interpolated per cell, reproducing the smooth sweep of
 * the Go renderer in the user's own colours. On a 256-colour theme
 * interpolation would land on cube indices the theme never chose, so each cell
 * instead goes through theme.fg with the nearest anchor's token — coarser, but
 * every colour on screen is one the theme actually declares.
 */
export function renderBar(fill: number, cells: number, style: string, theme: ThemeLike): string {
  if (cells <= 0) return "";
  const clamped = Math.max(0, Math.min(1, Number.isFinite(fill) ? fill : 0));
  const set = STYLES[style] ?? STYLES.block!;
  const edgeSteps = Math.max(1, set.edge.length - 1);
  const filled = clamped * cells;
  const whole = Math.floor(filled);
  const partial = filled - whole;

  let anchors: Array<[number, RGB]> | undefined;
  if (theme.getColorMode() === "truecolor") {
    const parsed: Array<[number, RGB]> = [];
    for (const [stop, token] of RAMP) {
      let rgb: RGB | undefined;
      try {
        rgb = parseFgAnsi(theme.getFgAnsi(token));
      } catch {
        rgb = undefined;
      }
      if (rgb) parsed.push([stop, rgb]);
    }
    if (parsed.length >= 2) anchors = parsed;
  }

  let out = "";
  for (let i = 0; i < cells; i++) {
    const t = (i + 0.5) / cells;
    let glyph: string;
    let lit = true;
    if (i < whole) {
      glyph = set.body;
    } else if (i === whole && partial > 0) {
      const idx = Math.min(edgeSteps, Math.max(1, Math.round(partial * edgeSteps)));
      glyph = set.edge[idx]!;
    } else {
      glyph = set.ghost;
      lit = false;
    }
    if (anchors) {
      const base = sampleRamp(t, anchors);
      out += truecolorCell(lit ? base : darken(base), glyph);
    } else {
      const token = lit ? tokenAt(t) : "dim";
      try {
        out += theme.fg(token, glyph);
      } catch {
        out += glyph;
      }
    }
  }
  return out;
}
