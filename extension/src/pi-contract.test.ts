import { describe, expect, it } from "bun:test";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

import { truncateToWidth, visibleWidth as piVisibleWidth } from "@earendil-works/pi-tui";

import { installStatusline } from "./component";
import { composeRow, renderRows } from "./layout";
import { parseSnapshot, type Snapshot } from "./snapshot";
import { recordingTheme } from "./testing";

const snap = parseSnapshot(
  readFileSync(join(import.meta.dir, "..", "testdata", "snapshot-full.json"), "utf8"),
) as Snapshot;
const NOW = 1748260800000;

/**
 * A verbatim copy of pi's sanitizeStatusText, from
 * src/modes/interactive/components/footer.ts:13-19. Every string handed to
 * ctx.ui.setStatus goes through this before it reaches the screen.
 *
 * It is copied rather than imported because it is module-private in pi. The
 * "stays in step with pi" test below reads pi's real source off disk and fails
 * if this body ever stops matching.
 */
function sanitizeStatusText(text: string): string {
  return text
    .replace(/[\r\n\t]/g, " ")
    .replace(/ +/g, " ")
    .trim();
}

/**
 * Locate pi's shipped footer source, preferring the TypeScript that the Nix
 * builds carry. The npm tarball ships only `dist/`, so the compiled JS is the
 * fallback — its sanitizeStatusText body is identical once whitespace is
 * normalised, which is all this comparison looks at. Point
 * PI_CODING_AGENT_SRC at a real pi's package root to check against the
 * TypeScript the user actually runs.
 */
function piFooterSource(): { path: string; body: string } | undefined {
  const roots = [
    process.env.PI_CODING_AGENT_SRC,
    join(import.meta.dir, "..", "node_modules", "@earendil-works", "pi-coding-agent"),
  ].filter(Boolean) as string[];
  const rels = [
    join("src", "modes", "interactive", "components", "footer.ts"),
    join("dist", "modes", "interactive", "components", "footer.js"),
  ];
  for (const root of roots) {
    for (const rel of rels) {
      const p = join(root, rel);
      if (existsSync(p)) return { path: p, body: readFileSync(p, "utf8") };
    }
  }
  return undefined;
}

describe("pi footer sink contract", () => {
  it("stays in step with pi's real sanitizeStatusText", () => {
    const found = piFooterSource();
    if (!found) {
      throw new Error(
        "pi source not found; install the pi-coding-agent devDependency or set " +
          "PI_CODING_AGENT_SRC to the bun-built pi's package root",
      );
    }
    // Matches both the .ts and the compiled .js form of the signature.
    const upstream = /function sanitizeStatusText\(text(?:: string)?\)(?:: string)? \{([^]*?)\n\}/.exec(
      found.body,
    );
    expect([found.path, upstream !== null]).toEqual([found.path, true]);
    // Normalise whitespace so tabs-vs-spaces cannot flap the assertion.
    const norm = (s: string) => s.replace(/\s+/g, " ").trim();
    expect(norm(upstream![1]!)).toBe(
      norm(`
        // Replace newlines, tabs, carriage returns with space, then collapse multiple spaces
        return text
          .replace(/[\\r\\n\\t]/g, " ")
          .replace(/ +/g, " ")
          .trim();
      `),
    );
  });

  it("proves our rows genuinely cannot survive that sink", () => {
    // Non-vacuity guard. If our rows happened to be sanitize-stable, the
    // assertion below would pass for the wrong reason and the regression it
    // guards could return unnoticed. They are not stable: multi-row output is
    // newline-joined, and flex spacers are runs of spaces.
    const { theme } = recordingTheme();
    const rows = renderRows(snap, 120, theme, NOW);
    expect(rows.length).toBeGreaterThan(1);
    const joined = rows.join("\n");
    expect(sanitizeStatusText(joined)).not.toBe(joined);
    expect(sanitizeStatusText(joined).includes("\n")).toBe(false);
  });

  it("never hands a row to setStatus", () => {
    // THE regression test. Phase 1 shipped `ctx.ui.setStatus(KEY, rows.join("\n"))`,
    // and this fails loudly on that code: the sink is instrumented with pi's own
    // sanitiser, and any non-undefined value that the sanitiser would alter is a
    // row being fed to a single-line, space-collapsing sink.
    const observed: Array<string | undefined> = [];
    const { theme } = recordingTheme();
    let widget: { render(width: number): string[] } | undefined;
    const ui = {
      setStatus: (_k: string, text: string | undefined) => {
        observed.push(text === undefined ? undefined : sanitizeStatusText(text));
        if (text !== undefined && sanitizeStatusText(text) !== text) {
          throw new Error(
            `setStatus was given text that pi would mangle:\n${JSON.stringify(text.slice(0, 200))}`,
          );
        }
      },
      setWidget: (_k: string, factory: any) => {
        widget = factory ? factory({ requestRender() {} }, theme) : undefined;
      },
      setFooter: (factory: any) => {
        if (factory) {
          factory({ requestRender() {} }, theme, {
            getGitBranch: () => "main",
            getExtensionStatuses: () => new Map(),
            getAvailableProviderCount: () => 1,
            onBranchChange: () => () => {},
          });
        }
      },
      theme,
    };
    const handle = installStatusline({ mode: "tui", hasUI: true, ui, cwd: "/x" } as any, {});
    handle.setSnapshot(snap);
    // Render too: the phase-1 bug lived on the output path, not the install
    // path, so a test that never draws a frame would not have seen it.
    widget?.render(120);
    handle.dispose();
    expect(observed.every((v) => v === undefined)).toBe(true);
  });
});

describe("pi widget render contract", () => {
  // The widget path applies no sanitisation and no truncation (pi mounts a
  // factory component verbatim), so the contract shifts entirely onto us:
  // every line must already fit. pi-tui's own functions are the authority on
  // both width and truncation.
  for (const width of [24, 40, 60, 80, 100, 120, 160, 200]) {
    it(`emits width-legal lines at ${width}`, () => {
      const { theme } = recordingTheme();
      const rows = renderRows(snap, width, theme, NOW);
      expect(rows.length).toBeGreaterThan(0);
      for (const line of rows) {
        expect([width, line, piVisibleWidth(line) <= width]).toEqual([width, line, true]);
        // Already-legal lines must be fixed points of pi's own truncator.
        expect(truncateToWidth(line, width)).toBe(line);
      }
    });
  }

  it("keeps rows as separate array entries, never newline-joined", () => {
    const { theme } = recordingTheme();
    for (const line of renderRows(snap, 120, theme, NOW)) {
      expect(line.includes("\n")).toBe(false);
      expect(line.includes("\r")).toBe(false);
      expect(line.includes("\t")).toBe(false);
    }
  });

  it("preserves flex padding, which the setStatus path destroyed", () => {
    const { theme } = recordingTheme();
    // composeRow with a real width budget is where a flex spacer means
    // anything. renderRows never gives it one: main.go composes both rows at
    // Width 0 to decide whether they merge, and its wrap path ignores flex
    // outright, so a flex spacer collapses to zero cells there. This port
    // keeps that behaviour rather than quietly improving on it, so the two
    // renderers still agree line for line.
    const line = composeRow(["model", snap.config.flexName, "cwd"], snap, 120, theme);
    expect(piVisibleWidth(line)).toBe(120);
    expect(/ {4,}/.test(line)).toBe(true);
    // And the proof this is exactly what the old path lost:
    expect(sanitizeStatusText(line)).not.toBe(line);
  });

  it("emits runs of spaces that the sink would collapse, at the width pi uses", () => {
    // Even without a flex spacer the rows carry padding the sanitiser eats —
    // this is the assertion that would still fail if flex were removed.
    const { theme } = recordingTheme();
    const joined = renderRows(snap, 40, theme, NOW).join("\n");
    expect(/ {2,}/.test(joined)).toBe(true);
    expect(sanitizeStatusText(joined)).not.toBe(joined);
  });
});
