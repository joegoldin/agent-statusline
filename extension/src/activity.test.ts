import { describe, expect, it } from "bun:test";

import { visibleWidth as piVisibleWidth } from "@earendil-works/pi-tui";

import { activityRows, spinnerFrame } from "./activity";
import type { Snapshot } from "./snapshot";
import { recordingTheme } from "./testing";

const NOW = 1748260800000;

function snapWith(activity: Partial<Snapshot["activity"]>): Snapshot {
  return {
    schema: 1,
    mode: "pi",
    asOfMs: NOW,
    config: {
      barWidth: 10,
      compactWidth: 70,
      activityRows: 4,
      hideWhenIdle: true,
      padding: 0,
      refreshIntervalMs: 1000,
      maxLines: 6,
      separator: " | ",
      flexName: "flex",
      row1: [],
      row2: [],
      hide: [],
      dropPriority: [],
    },
    widgets: {},
    activity: {
      graces: {
        toolCompleteMs: 30000,
        agentCompleteMs: 30000,
        agentRunningStaleMs: 1800000,
        todoCompleteMs: 60000,
      },
      tools: [],
      agents: [],
      todos: null,
      ...activity,
    },
  };
}

describe("spinnerFrame", () => {
  it("advances once per second and cycles", () => {
    const frames = [0, 1000, 2000, 3000, 4000].map((d) => spinnerFrame(NOW + d));
    expect(new Set(frames.slice(0, 4)).size).toBe(4);
    expect(frames[4]).toBe(frames[0]);
  });

  it("keeps every frame the same cell width, so nothing after it shifts", () => {
    const widths = new Set([0, 1000, 2000, 3000].map((d) => piVisibleWidth(spinnerFrame(NOW + d))));
    expect(widths.size).toBe(1);
  });
});

describe("activityRows", () => {
  it("computes elapsed against the render clock, not the snapshot", () => {
    const { theme } = recordingTheme();
    const snap = snapWith({
      tools: [
        { id: "c1", name: "bash", target: "bun test", state: "running", startedAtMs: NOW - 5000 },
      ],
    });
    const at5 = activityRows(snap, 200, theme, NOW, 4).join("");
    const at90 = activityRows(snap, 200, theme, NOW + 85_000, 4).join("");
    expect(at5).toContain("5s");
    expect(at90).toContain("1m30s");
  });

  it("drops a finished tool once its grace window expires, with no new snapshot", () => {
    const { theme } = recordingTheme();
    const snap = snapWith({
      tools: [
        { id: "c1", name: "read", state: "done", startedAtMs: NOW - 4000, endedAtMs: NOW - 1000 },
      ],
    });
    expect(activityRows(snap, 200, theme, NOW, 4).length).toBe(1);
    expect(activityRows(snap, 200, theme, NOW + 40_000, 4).length).toBe(0);
  });

  it("drops an agent stuck running past the staleness cap", () => {
    const { theme } = recordingTheme();
    const snap = snapWith({ agents: [{ name: "Explore", startedAtMs: NOW - 3_600_000 }] });
    expect(activityRows(snap, 200, theme, NOW, 4).length).toBe(0);
  });

  it("shows 3 tools on a wide terminal and 2 on a normal one", () => {
    const { theme } = recordingTheme();
    const tools = ["a", "b", "c", "d"].map((id) => ({
      id,
      name: "bash",
      target: `command-${id}`,
      state: "running" as const,
      startedAtMs: NOW - 1000,
    }));
    const snap = snapWith({ tools });
    const wide = activityRows(snap, 130, theme, NOW, 4)[0]!;
    const normal = activityRows(snap, 80, theme, NOW, 4)[0]!;
    expect(wide.match(/command-/g)?.length).toBe(3);
    expect(normal.match(/command-/g)?.length).toBe(2);
  });

  it("middle-truncates each tool to its share of the line", () => {
    const { theme } = recordingTheme();
    const snap = snapWith({
      tools: [
        {
          id: "c1",
          name: "bash",
          state: "running",
          startedAtMs: NOW - 1000,
          target: "nix shell nixpkgs#go --command go test ./internal/widgets/ -run TestSpans -v",
        },
      ],
    });
    const row = activityRows(snap, 60, theme, NOW, 4)[0]!;
    expect(piVisibleWidth(row)).toBeLessThanOrEqual(60);
    expect(row).toContain("…");
  });

  it("respects the row budget", () => {
    const { theme } = recordingTheme();
    const snap = snapWith({
      tools: [{ id: "c1", name: "bash", state: "running", startedAtMs: NOW }],
      agents: [{ name: "Explore", startedAtMs: NOW }],
      todos: { subject: "ship it", done: 1, total: 3, allComplete: false, timestampMs: NOW },
    });
    // Three rows deep now: running tools, agents, todos.
    expect(activityRows(snap, 200, theme, NOW, 4).length).toBe(3);
    expect(activityRows(snap, 200, theme, NOW, 2).length).toBe(2);
    expect(activityRows(snap, 200, theme, NOW, 0).length).toBe(0);
  });

  it("marks waiting tools with the queued glyph and dims them", () => {
    const rec = recordingTheme();
    const snap = snapWith({
      tools: [{ id: "q", name: "read", state: "waiting", emittedAtMs: NOW - 2000 }],
    });
    activityRows(snap, 200, rec.theme, NOW, 4);
    expect(rec.tokens()).toContain("dim");
  });

  it("drops an all-complete todo once its celebration is over", () => {
    const { theme } = recordingTheme();
    const snap = snapWith({
      todos: { done: 3, total: 3, allComplete: true, timestampMs: NOW - 1000 },
    });
    expect(activityRows(snap, 200, theme, NOW, 4).length).toBe(1);
    expect(activityRows(snap, 200, theme, NOW + 90_000, 4).length).toBe(0);
  });

  it("tolerates null activity arrays from an older snapshot", () => {
    const { theme } = recordingTheme();
    const snap = snapWith({ tools: null, agents: null });
    expect(activityRows(snap, 200, theme, NOW, 4)).toEqual([]);
  });

  it("emits lines that fit, measured by pi-tui, at every width", () => {
    const { theme } = recordingTheme();
    const snap = snapWith({
      tools: [
        { id: "c1", name: "bash", target: "a".repeat(200), state: "running", startedAtMs: NOW - 1 },
        { id: "c2", name: "read", target: "b".repeat(200), state: "running", startedAtMs: NOW - 1 },
      ],
      agents: [{ name: "Explore", model: "sol", description: "c".repeat(200), startedAtMs: NOW }],
      todos: { subject: "d".repeat(200), done: 1, total: 3, allComplete: false, timestampMs: NOW },
    });
    for (const width of [12, 20, 40, 80, 120, 200]) {
      for (const line of activityRows(snap, width, theme, NOW, 4)) {
        expect([width, piVisibleWidth(line) <= width]).toEqual([width, true]);
      }
    }
  });
});
