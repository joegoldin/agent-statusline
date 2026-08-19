import { describe, expect, it } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { visibleWidth as piVisibleWidth } from "@earendil-works/pi-tui";

import { composeRow, renderRows, wrapRow } from "./layout";
import { parseSnapshot, type Snapshot } from "./snapshot";
import { recordingTheme } from "./testing";

const snap = parseSnapshot(
  readFileSync(join(import.meta.dir, "..", "testdata", "snapshot-full.json"), "utf8"),
) as Snapshot;

/**
 * The two single-widget rows, injected rather than taken from the fixture, so
 * these assertions describe the layout rule and not whichever widgets happened
 * to be visible when snapshot-full.json was regenerated.
 */
function withExtraRows(base: Snapshot): Snapshot {
  return {
    ...base,
    config: { ...base.config, row3: ["autoMode"], row4: ["cache"] },
    widgets: {
      ...base.widgets,
      autoMode: { visible: true, spans: [{ kind: "text", intent: "ok", text: "auto \u25cf" }] },
      cache: { visible: true, spans: [{ kind: "text", intent: "ok", text: "cache 79.3%" }] },
    },
  };
}

/** The same snapshot with both extra rows switched off in config. */
function withoutExtraRows(base: Snapshot): Snapshot {
  return { ...base, config: { ...base.config, row3: [], row4: [] } };
}

const NOW = 1748260800000;

describe("composeRow", () => {
  it("joins visible widgets with the snapshot's separator", () => {
    const { theme } = recordingTheme();
    const row = composeRow(["model", "cwd"], snap, 200, theme);
    expect(row).toContain(snap.config.separator.trim());
  });

  it("skips widgets marked invisible and collapses their separators", () => {
    const { theme } = recordingTheme();
    const hidden: Snapshot = {
      ...snap,
      widgets: { ...snap.widgets, cwd: { visible: false } },
    };
    const row = composeRow(["model", "cwd", "duration"], hidden, 200, theme);
    const seps = row.split(snap.config.separator).length - 1;
    expect(seps).toBe(1);
  });

  it("honours config.hide regardless of row membership", () => {
    const { theme } = recordingTheme();
    const hidden: Snapshot = { ...snap, config: { ...snap.config, hide: ["cwd"] } };
    expect(composeRow(["model", "cwd"], hidden, 200, theme)).not.toContain(snap.config.separator);
  });

  it("expands a flex marker to fill the remaining width", () => {
    const { theme } = recordingTheme();
    const row = composeRow(["model", snap.config.flexName, "cwd"], snap, 100, theme);
    expect(piVisibleWidth(row)).toBe(100);
  });

  it("puts no separator beside a flex spacer, which is itself the gap", () => {
    const { theme } = recordingTheme();
    const row = composeRow(["model", snap.config.flexName, "cwd"], snap, 100, theme);
    expect(row.split(snap.config.separator).length - 1).toBe(0);
  });

  it("drops the lowest-priority widget first on overflow", () => {
    const { theme } = recordingTheme();
    const names = ["model", "cwd", "git", "duration"];
    const wide = composeRow(names, snap, 300, theme);
    const narrow = composeRow(names, snap, 30, theme);
    expect(piVisibleWidth(narrow)).toBeLessThanOrEqual(30);
    expect(narrow.length).toBeLessThan(wide.length);
  });

  it("selects the compact span list below config.compactWidth", () => {
    const { theme } = recordingTheme();
    const wide = composeRow(["context"], snap, 200, theme);
    const narrow = composeRow(["context"], snap, snap.config.compactWidth - 1, theme);
    expect(narrow).not.toBe(wide);
    // The compact context form drops the bar, so it is strictly narrower.
    expect(piVisibleWidth(narrow)).toBeLessThan(piVisibleWidth(wide));
  });
});

describe("wrapRow", () => {
  it("packs segments across lines rather than dropping them", () => {
    const { theme } = recordingTheme();
    const lines = wrapRow(snap.config.row1, snap, 40, theme);
    expect(lines.length).toBeGreaterThan(1);
    for (const l of lines) expect(piVisibleWidth(l)).toBeLessThanOrEqual(40);
  });
});

describe("renderRows", () => {
  // The contract pi enforces: every returned line fits the width it pushed in.
  // Measured with pi-tui's own visibleWidth, not ours, so this is pi's verdict.
  for (const width of [20, 40, 60, 80, 120, 200]) {
    it(`returns only lines that fit at width ${width}`, () => {
      const { theme } = recordingTheme();
      const rows = renderRows(snap, width, theme, NOW);
      expect(rows.length).toBeGreaterThan(0);
      for (const line of rows) {
        expect([width, line, piVisibleWidth(line) <= width]).toEqual([width, line, true]);
      }
    });
  }

  it("never exceeds config.maxLines", () => {
    const { theme } = recordingTheme();
    expect(renderRows(snap, 40, theme, NOW).length).toBeLessThanOrEqual(snap.config.maxLines);
  });

  it("returns no empty rows, which would leave blank gaps in the dock", () => {
    const { theme } = recordingTheme();
    for (const line of renderRows(snap, 120, theme, NOW)) {
      expect(line.trim().length).toBeGreaterThan(0);
    }
  });

  it("merges the two dashboard rows when they fit on one line", () => {
    const { theme } = recordingTheme();
    const wide = renderRows(snap, 400, theme, NOW);
    const narrow = renderRows(snap, 40, theme, NOW);
    expect(wide.length).toBeLessThan(narrow.length);
  });

  it("uses only theme tokens, so switching themes changes the output", () => {
    const a = recordingTheme();
    const b = recordingTheme({
      ansi: {
        text: "\x1b[38;2;0;0;0m",
        dim: "\x1b[38;2;1;1;1m",
        muted: "\x1b[38;2;2;2;2m",
        accent: "\x1b[38;2;3;3;3m",
        success: "\x1b[38;2;4;4;4m",
        warning: "\x1b[38;2;5;5;5m",
        error: "\x1b[38;2;6;6;6m",
        mdLink: "\x1b[38;2;7;7;7m",
        customMessageLabel: "\x1b[38;2;8;8;8m",
      },
    });
    const ra = renderRows(snap, 120, a.theme, NOW);
    const rb = renderRows(snap, 120, b.theme, NOW);
    // Same structure, different bar colours: the ramp came from the theme.
    expect(ra.length).toBe(rb.length);
    expect(ra.join("\n")).not.toBe(rb.join("\n"));
    expect(a.tokens().length).toBeGreaterThan(3);
  });

  it("survives a snapshot with no widgets at all", () => {
    const { theme } = recordingTheme();
    const empty: Snapshot = {
      ...snap,
      widgets: {},
      activity: { ...snap.activity, tools: [], toolCounts: [], agents: [], todos: null },
    };
    expect(renderRows(empty, 80, theme, NOW)).toEqual([]);
  });

  it("keeps the clocks live between snapshots", () => {
    // The whole reason the component owns a tick: nothing about the snapshot
    // changes here, only the render clock, and the output must still move.
    const { theme } = recordingTheme();
    const at0 = renderRows(snap, 120, theme, NOW).join("\n");
    const at7 = renderRows(snap, 120, theme, NOW + 7000).join("\n");
    expect(at7).not.toBe(at0);
  });

  it("draws row3 and row4 as extra lines under the dashboard", () => {
    const { theme } = recordingTheme();
    const rows = renderRows(withExtraRows(snap), 400, theme, NOW);
    // Width 400 merges the dashboard onto one line, so the extra rows are the
    // two immediately after it and the activity stack follows them.
    expect(rows[1]).toContain("auto \u25cf");
    expect(rows[2]).toContain("cache 79.3%");
  });

  it("costs no line when an extra row's widget is hidden", () => {
    const { theme } = recordingTheme();
    const withRows = withExtraRows(snap);
    const hidden: Snapshot = {
      ...withRows,
      widgets: { ...withRows.widgets, autoMode: { visible: false }, cache: { visible: false } },
    };
    expect(renderRows(hidden, 400, theme, NOW)).toEqual(renderRows(withoutExtraRows(snap), 400, theme, NOW));
  });

  it("honours the hide list for the extra rows", () => {
    const { theme } = recordingTheme();
    const withRows = withExtraRows(snap);
    const hidden: Snapshot = { ...withRows, config: { ...withRows.config, hide: ["autoMode", "cache"] } };
    expect(renderRows(hidden, 400, theme, NOW)).toEqual(renderRows(withoutExtraRows(snap), 400, theme, NOW));
  });
});
