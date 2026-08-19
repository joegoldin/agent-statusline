import { describe, expect, it } from "bun:test";

import { renderBar } from "./bars";
import { recordingTheme } from "./testing";
import { visibleWidth } from "./width";

describe("renderBar", () => {
  it("is exactly `cells` columns wide at every fill, in both colour modes", () => {
    for (const colorMode of ["truecolor", "256color"] as const) {
      const { theme } = recordingTheme({ colorMode });
      for (const fill of [0, 0.01, 0.25, 0.5, 0.535, 0.99, 1]) {
        for (const style of ["braille", "block", "line"] as const) {
          expect([colorMode, fill, style, visibleWidth(renderBar(fill, 10, style, theme))]).toEqual([
            colorMode,
            fill,
            style,
            10,
          ]);
        }
      }
    }
  });

  it("derives its ramp from theme tokens, never from a hardcoded palette", () => {
    const rec = recordingTheme();
    renderBar(0.6, 10, "block", rec.theme);
    expect(rec.tokens().sort()).toEqual(["dim", "error", "success", "warning"]);
  });

  it("interpolates per cell on a truecolor theme", () => {
    const { theme } = recordingTheme({ colorMode: "truecolor" });
    const out = renderBar(1, 10, "block", theme);
    const colours = new Set(out.match(/38;2;\d+;\d+;\d+/g) ?? []);
    expect(colours.size).toBeGreaterThan(4);
  });

  it("uses discrete theme colours on a 256-colour theme", () => {
    const rec = recordingTheme({ colorMode: "256color" });
    const out = renderBar(1, 10, "block", rec.theme);
    // No interpolated truecolor sequences; every cell went through theme.fg.
    expect(out).not.toMatch(/38;2;/);
    expect(rec.calls().filter(([, text]) => text !== "").length).toBeGreaterThan(0);
  });

  it("repaints when the theme changes, which is the whole point", () => {
    const a = recordingTheme();
    const b = recordingTheme({
      ansi: {
        dim: "\x1b[38;2;10;10;10m",
        success: "\x1b[38;2;20;20;20m",
        warning: "\x1b[38;2;30;30;30m",
        error: "\x1b[38;2;40;40;40m",
      },
    });
    expect(renderBar(0.6, 10, "block", a.theme)).not.toBe(renderBar(0.6, 10, "block", b.theme));
  });

  it("clamps out-of-range fills instead of throwing", () => {
    const { theme } = recordingTheme();
    expect(visibleWidth(renderBar(-1, 6, "block", theme))).toBe(6);
    expect(visibleWidth(renderBar(9, 6, "block", theme))).toBe(6);
    expect(visibleWidth(renderBar(Number.NaN, 6, "block", theme))).toBe(6);
  });

  it("returns empty for a zero-width bar", () => {
    const { theme } = recordingTheme();
    expect(renderBar(0.5, 0, "block", theme)).toBe("");
  });

  it("falls back to block glyphs for an unknown style", () => {
    const { theme } = recordingTheme();
    expect(visibleWidth(renderBar(0.5, 8, "no-such-style", theme))).toBe(8);
  });
});
