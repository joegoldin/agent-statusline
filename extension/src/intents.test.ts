import { describe, expect, it } from "bun:test";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

import { INTENT_TOKENS, paint } from "./intents";
import type { SnapshotConfig, Span } from "./snapshot";
import { recordingTheme } from "./testing";

const cfg = { barWidth: 10, separator: " | " } as SnapshotConfig;

// Exactly the intents internal/render/span.go can emit. If Go grows one and
// this list is not updated, the mapping test below fails rather than silently
// painting it in the default foreground.
const GO_INTENTS = [
  "text",
  "dim",
  "muted",
  "accent",
  "meta",
  "path",
  "ok",
  "warn",
  "caution",
  "danger",
] as const;

describe("INTENT_TOKENS", () => {
  it("maps every intent Go can emit", () => {
    for (const i of GO_INTENTS) {
      expect(INTENT_TOKENS[i]).toBeDefined();
      expect(typeof INTENT_TOKENS[i].token).toBe("string");
    }
    expect(Object.keys(INTENT_TOKENS).sort()).toEqual([...GO_INTENTS].sort());
  });

  it("uses only real pi theme colour slots", () => {
    // From ThemeColor in pi's src/modes/interactive/theme/theme.ts:112-159.
    const SLOTS = new Set([
      "accent",
      "border",
      "borderAccent",
      "borderMuted",
      "success",
      "error",
      "warning",
      "muted",
      "dim",
      "text",
      "thinkingText",
      "searchMatchText",
      "userMessageText",
      "customMessageText",
      "customMessageLabel",
      "toolTitle",
      "toolOutput",
      "mdHeading",
      "mdLink",
      "mdLinkUrl",
      "mdCode",
    ]);
    for (const i of GO_INTENTS) {
      expect([i, SLOTS.has(INTENT_TOKENS[i].token)]).toEqual([i, true]);
    }
  });

  it("splits path from warn, which Go cannot", () => {
    expect(INTENT_TOKENS.path.token).not.toBe(INTENT_TOKENS.warn.token);
  });

  it("expresses caution as a bolded warning, the closest a theme can get", () => {
    expect(INTENT_TOKENS.caution.token).toBe("warning");
    expect(INTENT_TOKENS.caution.bold).toBe(true);
  });
});

describe("paint", () => {
  it("routes every text span through theme.fg, never through a literal escape", () => {
    const rec = recordingTheme();
    const spans: Span[] = [
      { kind: "text", text: "main", intent: "ok" },
      { kind: "text", text: " ", intent: "text" },
      { kind: "text", text: "60%", intent: "caution" },
    ];
    const out = paint(spans, rec.theme, cfg);
    expect(out).not.toMatch(/\x1b\[3[0-9]m/);
    expect(rec.tokens().sort()).toEqual(["success", "text", "warning"]);
  });

  it("bolds the caution intent", () => {
    const rec = recordingTheme();
    const out = paint([{ kind: "text", text: "60%", intent: "caution" }], rec.theme, cfg);
    expect(out).toContain("<b>");
  });

  it("treats a missing intent as text rather than dropping the span", () => {
    const rec = recordingTheme();
    expect(paint([{ kind: "text", text: "raw" }], rec.theme, cfg)).toContain("raw");
  });

  it("emits an empty string for empty text, with no dangling escapes", () => {
    const rec = recordingTheme();
    expect(paint([{ kind: "text", text: "", intent: "ok" }], rec.theme, cfg)).toBe("");
  });

  it("wraps a linked span in OSC 8 outside the colour", () => {
    const rec = recordingTheme();
    const out = paint(
      [{ kind: "text", text: "#12", intent: "accent", link: "https://x/1" }],
      rec.theme,
      cfg,
    );
    expect(out.startsWith("\x1b]8;;https://x/1\x1b\\")).toBe(true);
    expect(out.endsWith("\x1b]8;;\x1b\\")).toBe(true);
    expect(out).toContain("#12");
  });

  it("renders a bar span through the bar renderer, not as text", () => {
    const rec = recordingTheme();
    const out = paint([{ kind: "bar", fill: 0.5, cells: 4, style: "block" }], rec.theme, cfg);
    expect(out.length).toBeGreaterThan(0);
    expect(rec.tokens()).toContain("success");
  });

  it("survives an unknown intent from a newer binary", () => {
    const rec = recordingTheme();
    const span = { kind: "text", text: "x", intent: "brand-new" } as unknown as Span;
    expect(paint([span], rec.theme, cfg)).toContain("x");
  });

  it("survives a theme missing an optional slot", () => {
    // pi's Theme.fg throws on an unknown colour (theme.ts:390-394). A widget
    // must lose its colour, never itself.
    const rec = recordingTheme({ ansi: { text: "\x1b[38;2;1;1;1m" } });
    const bare = {
      ...rec.theme,
      fg: (color: string, text: string) => {
        if (color !== "text") throw new Error(`Unknown theme color: ${color}`);
        return text;
      },
    };
    expect(paint([{ kind: "text", text: "#12", intent: "meta" }], bare, cfg)).toBe("#12");
  });
});

describe("runtime sources", () => {
  // The two rules that keep this extension loadable and themeable, enforced
  // mechanically rather than by review.
  const runtimeFiles = () =>
    readdirSync(import.meta.dir).filter(
      (f) => f.endsWith(".ts") && !f.endsWith(".test.ts") && f !== "testing.ts",
    );

  it("contain no literal SGR colour codes outside the bar renderer", () => {
    // Resets are structure, not colour: pi's own truncator emits \x1b[0m and
    // its fg() closes with \x1b[39m, and width.ts is a port of both. A colour
    // literal is anything else.
    const RESETS = new Set(["", "0", "22", "39", "49"]);
    const offenders: string[] = [];
    for (const f of runtimeFiles()) {
      if (f === "bars.ts") continue; // composes truecolor from parsed theme anchors
      const body = readFileSync(join(import.meta.dir, f), "utf8");
      for (const m of body.matchAll(/\\(?:x1b|u001b)\[([0-9;]*)m/g)) {
        if (!RESETS.has(m[1]!)) offenders.push(`${f}: ESC[${m[1]}m`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it("import nothing from node_modules at runtime", () => {
    const offenders: string[] = [];
    for (const f of runtimeFiles()) {
      const body = readFileSync(join(import.meta.dir, f), "utf8");
      for (const m of body.matchAll(/from\s+"([^"]+)"/g)) {
        const spec = m[1]!;
        if (!spec.startsWith(".") && !spec.startsWith("node:")) offenders.push(`${f}: ${spec}`);
      }
    }
    expect(offenders).toEqual([]);
  });
});
