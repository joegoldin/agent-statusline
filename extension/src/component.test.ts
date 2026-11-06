import { describe, expect, it, jest, mock } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { installStatusline, WIDGET_KEY } from "./component";
import { parseSnapshot, type Snapshot } from "./snapshot";
import { recordingTheme } from "./testing";

const snap = parseSnapshot(
  readFileSync(join(import.meta.dir, "..", "testdata", "snapshot-full.json"), "utf8"),
) as Snapshot;

/** A pi UI double capturing the factories, so tests can mount them by hand. */
function fakeUI(extensionStatuses = new Map<string, string>()) {
  const widgets = new Map<string, { factory: any; options: any; component?: any }>();
  let footerComponent: any;
  const requestRender = mock(() => {});
  const tui = { requestRender };
  const { theme, tokens, calls } = recordingTheme();
  const branchListeners: Array<() => void> = [];
  const footerData = {
    getGitBranch: () => "main",
    getExtensionStatuses: () => extensionStatuses,
    getAvailableProviderCount: () => 1,
    onBranchChange: (cb: () => void) => {
      branchListeners.push(cb);
      return () => {
        const i = branchListeners.indexOf(cb);
        if (i >= 0) branchListeners.splice(i, 1);
      };
    },
  };
  const setStatusCalls: Array<[string, string | undefined]> = [];
  const ui = {
    setStatus(key: string, text: string | undefined) {
      setStatusCalls.push([key, text]);
    },
    setWidget(key: string, factory: any, options: any) {
      const prev = widgets.get(key);
      prev?.component?.dispose?.();
      if (factory === undefined) {
        widgets.delete(key);
        return;
      }
      widgets.set(key, { factory, options, component: factory(tui, theme) });
    },
    setFooter(factory: any) {
      footerComponent?.dispose?.();
      footerComponent = factory === undefined ? undefined : factory(tui, theme, footerData);
    },
    theme,
  };
  return {
    ctx: { mode: "tui", hasUI: true, ui, cwd: "/home/joe/p" },
    widgets,
    requestRender,
    tokens,
    calls,
    setStatusCalls,
    footer: () => footerComponent,
    branchChange: () => branchListeners.forEach((cb) => cb()),
    branchListenerCount: () => branchListeners.length,
  };
}

describe("installStatusline", () => {
  it("uses the native footer so Lens and Jobs always precede the dashboard", () => {
    const f = fakeUI();
    const handle = installStatusline(f.ctx as any, {});
    handle.setSnapshot(snap);
    expect(f.widgets.size).toBe(0);
    expect(f.footer().render(120).length).toBeGreaterThan(0);
    handle.dispose();
  });

  it("renders fast once after model and effort, in a distinct color, and clears it immediately", () => {
    const statuses = new Map([["pi-fast", "fast"]]);
    const f = fakeUI(statuses);
    const h = installStatusline(f.ctx as any, {});
    const model = { visible: true, spans: [{ kind: "text" as const, intent: "accent" as const, text: "Sol xhigh" }],
      compact: [{ kind: "text" as const, intent: "accent" as const, text: "Sol xh" }] };
    const fixture = { ...snap, widgets: { ...snap.widgets, model },
      config: { ...snap.config, row1: ["model"], row2: [], row3: [], row4: [], padding: 0, activityRows: 0 } };
    h.setSnapshot(fixture);
    const render = (width: number) => f.footer().render(width).join("\n");
    const plain = (value: string) => value.replace(/\x1b\[[0-9;]*m/g, "");
    try {
      expect(plain(render(200))).toContain("Sol xhigh fast");
      expect(plain(render(200)).match(/fast/g)).toHaveLength(1);
      expect(f.calls()).toContainEqual(["warning", " fast"]);
      expect(f.calls()).toContainEqual(["accent", "Sol xhigh"]);
      expect(plain(render(60))).toContain("Sol xh fast");
      expect(model.spans).toHaveLength(1);
      expect(model.compact).toHaveLength(1);
      statuses.delete("pi-fast");
      expect(plain(render(200))).not.toContain("fast");
    } finally { h.dispose(); }
  });

  it("keeps drawing the dashboard if the fast-status lookup fails", () => {
    const statuses = new Map<string, string>();
    statuses.get = () => { throw new Error("status unavailable"); };
    const f = fakeUI(statuses);
    const h = installStatusline(f.ctx as any, {});
    h.setSnapshot(snap);
    try {
      const rows = f.footer().render(200);
      expect(rows.join("\n")).toContain(snap.widgets.model.spans![0].text!);
    } finally { h.dispose(); }
  });

  it("never routes rows through setStatus", () => {
    const f = fakeUI();
    const h = installStatusline(f.ctx as any, {});
    h.setSnapshot(snap);
    f.footer().render(120);
    for (const [, text] of f.setStatusCalls) {
      expect(text).toBeUndefined();
    }
  });

  it("does nothing outside TUI mode", () => {
    const f = fakeUI();
    installStatusline({ ...f.ctx, mode: "print", hasUI: false } as any, {});
    expect(f.widgets.size).toBe(0);
    expect(f.footer()).toBeUndefined();
  });

  it("renders nothing until a snapshot arrives", () => {
    const f = fakeUI();
    installStatusline(f.ctx as any, {});
    expect(f.footer().render(120)).toEqual([]);
  });

  it("renders rows once a snapshot is set", () => {
    const f = fakeUI();
    const h = installStatusline(f.ctx as any, {});
    h.setSnapshot(snap);
    expect(f.footer().render(120).length).toBeGreaterThan(0);
  });

  it("appends other extensions' status lines, which we displaced", () => {
    const f = fakeUI(
      new Map([
        ["zz-other", "other line one\nother line two"],
        [WIDGET_KEY, "stale"],
      ]),
    );
    const h = installStatusline(f.ctx as any, {});
    h.setSnapshot(snap);
    const rows = f.footer().render(200).join("\n");
    expect(rows).toContain("other line one");
    expect(rows).toContain("other line two");
    expect(rows).not.toContain("stale");
  });

  it("skips the status lines a widget already draws, and only those", () => {
    const f = fakeUI(
      new Map([
        ["pi-lens-lsp", "LSP Active: gopls"],
        ["usage", "chatgpt plan 56% \u21bb 3d5h"],
        ["zz-other", "other line"],
      ]),
    );
    let absorbed = new Set(["pi-lens-lsp", "usage"]);
    const h = installStatusline(f.ctx as any, { absorbedStatusKeys: () => absorbed });
    h.setSnapshot(snap);
    const render = () => f.footer().render(200).join("\n");
    expect(render()).not.toContain("LSP Active");
    expect(render()).not.toContain("chatgpt plan");
    expect(render()).toContain("other line");

    // Asked per frame: once the usage widgets stop showing pi-usage's plan,
    // its line comes back without a reinstall.
    absorbed = new Set(["pi-lens-lsp"]);
    expect(render()).toContain("chatgpt plan");
  });

  it("keeps every foreign line when the absorbed-key callback throws", () => {
    const f = fakeUI(new Map([["usage", "openrouter $4.20 left"]]));
    const h = installStatusline(f.ctx as any, {
      absorbedStatusKeys: () => {
        throw new Error("boom");
      },
    });
    h.setSnapshot(snap);
    expect(f.footer().render(200).join("\n")).toContain("openrouter $4.20 left");
  });

  it("reads another extension's status text through the footer it took", () => {
    const f = fakeUI(new Map([["pi-lens-lsp", "LSP Active: gopls"], ["empty", ""]]));
    const h = installStatusline(f.ctx as any, {});
    expect(h.extensionStatus("pi-lens-lsp")).toBe("LSP Active: gopls");
    expect(h.extensionStatus("empty")).toBeUndefined();
    expect(h.extensionStatus("absent")).toBeUndefined();
  });

  it("reads no status text outside TUI mode", () => {
    const f = fakeUI(new Map([["pi-lens-lsp", "LSP Active: gopls"]]));
    const h = installStatusline({ ...f.ctx, mode: "print", hasUI: false } as any, {});
    expect(h.extensionStatus("pi-lens-lsp")).toBeUndefined();
  });

  it("ticks requestRender at the snapshot's refresh interval", () => {
    jest.useFakeTimers();
    const f = fakeUI();
    const h = installStatusline(f.ctx as any, {});
    h.setSnapshot(snap);
    f.requestRender.mockClear();
    jest.advanceTimersByTime(5000);
    expect(f.requestRender).toHaveBeenCalledTimes(5);
    h.dispose();
    jest.useRealTimers();
  });

  it("re-arms the tick when the config's interval changes", () => {
    jest.useFakeTimers();
    const f = fakeUI();
    const h = installStatusline(f.ctx as any, {});
    h.setSnapshot({ ...snap, config: { ...snap.config, refreshIntervalMs: 500 } });
    f.requestRender.mockClear();
    jest.advanceTimersByTime(2000);
    expect(f.requestRender).toHaveBeenCalledTimes(4);
    h.dispose();
    jest.useRealTimers();
  });

  it("stops ticking after dispose and leaves no timers behind", () => {
    jest.useFakeTimers();
    const before = jest.getTimerCount();
    const f = fakeUI();
    const h = installStatusline(f.ctx as any, {});
    h.setSnapshot(snap);
    expect(jest.getTimerCount()).toBeGreaterThan(before);
    h.dispose();
    f.requestRender.mockClear();
    jest.advanceTimersByTime(10_000);
    expect(f.requestRender).not.toHaveBeenCalled();
    expect(jest.getTimerCount()).toBe(before);
    jest.useRealTimers();
  });

  it("restores the native footer on dispose", () => {
    const f = fakeUI();
    const h = installStatusline(f.ctx as any, {});
    h.dispose();
    expect(f.widgets.has(WIDGET_KEY)).toBe(false);
    expect(f.footer()).toBeUndefined();
  });

  it("unsubscribes from branch changes on dispose", () => {
    const f = fakeUI();
    const h = installStatusline(f.ctx as any, {});
    expect(f.branchListenerCount()).toBe(1);
    h.dispose();
    expect(f.branchListenerCount()).toBe(0);
  });

  it("asks for a data refresh when the branch changes", () => {
    const onDataStale = mock(() => {});
    const f = fakeUI();
    installStatusline(f.ctx as any, { onDataStale });
    f.branchChange();
    expect(onDataStale).toHaveBeenCalled();
  });

  it("re-installing on the same key disposes the previous component", () => {
    jest.useFakeTimers();
    const f = fakeUI();
    const first = installStatusline(f.ctx as any, {});
    first.setSnapshot(snap);
    const count = jest.getTimerCount();
    const second = installStatusline(f.ctx as any, {});
    second.setSnapshot(snap);
    expect(jest.getTimerCount()).toBe(count);
    second.dispose();
    jest.useRealTimers();
  });

  it("keeps the last good snapshot when a render throws", () => {
    const f = fakeUI();
    const h = installStatusline(f.ctx as any, {});
    h.setSnapshot(snap);
    const good = f.footer().render(120);
    // A snapshot parseSnapshot would have refused. The component is the last
    // line of defence: a statusline must never take the session down with it.
    h.setSnapshot({ ...snap, config: null as any });
    expect(f.footer().render(120)).toEqual(good);
  });

  it("repaints when the theme changes, via invalidate", () => {
    const f = fakeUI();
    const h = installStatusline(f.ctx as any, {});
    h.setSnapshot(snap);
    f.requestRender.mockClear();
    f.footer().invalidate();
    expect(f.requestRender).toHaveBeenCalled();
  });
});

it("keeps performance visible once in docked and narrow wrapping layouts", () => {
  const f = fakeUI();
  let badge = "\uf0172.1s \uf0e782t/s";
  const h = installStatusline(f.ctx as any, { performance: () => [{ kind: "text", intent: "accent", text: badge }] });
  h.setSnapshot(snap);
  const plain = (value: string) => value.replace(/\x1b\[[0-9;]*m/g, "");
  try {
    for (const width of [30, 60, 120, 200]) {
      const rows = f.footer().render(width).map(plain);
      expect(rows.join("\n")).toContain(badge);
      expect(rows[0].length).toBeLessThanOrEqual(width);
      expect(rows.join("\n").split(badge)).toHaveLength(2);
      expect(rows.length).toBeLessThanOrEqual(snap.config.maxLines);
    }
    expect(f.footer().render(8).join("")).not.toContain(badge);
    badge = "";
    expect(f.footer().render(120).join("")).not.toContain("82t/s");
    f.requestRender.mockClear();
    h.requestRender();
    expect(f.requestRender).toHaveBeenCalledTimes(1);
  } finally { h.dispose(); }
});

it("packs extension chips into available dashboard space", () => {
  const f = fakeUI(new Map([["background-tasks", "⚙ bg 2↻ · 1✗"], ["automode", "auto 3/100"]]));
  const h = installStatusline(f.ctx as any, { performance: () => [{ kind: "text", text: "80t/s" }] });
  h.setSnapshot({ ...snap, config: { ...snap.config, row1: ["model"], row2: [], row3: [], row4: [], padding: 0 }, activity: { ...snap.activity, tools: [], agents: [], todos: null } });
  try {
    const rows = f.footer().render(120);
    expect(rows).toHaveLength(1);
    expect(rows[0]).toContain("bg 2↻");
    expect(rows[0]).toContain("auto 3/100");
    expect(rows[0].replace(/\x1b\[[0-9;]*m/g, "").endsWith("80t/s")).toBe(true);
  } finally { h.dispose(); }
});

it("packs an inline Lens chip with left widgets and routes only its painted cells", () => {
  const plain = (text: string) => text.replace(/\x1b\[[0-9;]*m/g, "");
  const registry = ((globalThis as any)[Symbol.for("pi.inline-status-widgets.v1")] ??= { widgets: new Map(), consumers: new Set() });
  const click = mock(() => ({ handled: true, render: true }));
  registry.widgets.set("pi-lens", { render: () => "◇ lens ✓ clean", handleMouse: click });
  const f = fakeUI();
  const h = installStatusline(f.ctx as any, { performance: () => [{ kind: "text", text: "80t/s" }] });
  h.setSnapshot({ ...snap, widgets: { model: { visible: true, spans: [{ kind: "text", text: "MODEL" }] }, context: { visible: true, spans: [{ kind: "text", text: "CTX70%" }] } }, config: { ...snap.config, row1: ["model"], row2: ["context"], row3: [], row4: [], padding: 0, activityRows: 0 } });
  try {
    const rows = f.footer().render(80).map(plain);
    expect(rows).toHaveLength(2);
    expect(rows[0]).toContain("MODEL │ ◇ lens ✓ clean");
    expect(f.footer().handleMouse({ type: "click", button: "left", x: 0, y: 0 })).toBeUndefined();
    expect(click).not.toHaveBeenCalled();
    const x = rows[0].indexOf("◇ lens");
    expect(f.footer().handleMouse({ type: "click", button: "left", x, y: 0 }).handled).toBe(true);
    expect(click).toHaveBeenCalledTimes(1);
    expect(f.footer().handleMouse({ type: "click", button: "left", x, y: 1 })).toBeUndefined();
    const narrow = f.footer().render(16).map(plain);
    expect(narrow.some((row: string) => row.includes("◇ lens"))).toBe(true);
    const y = narrow.findIndex((row: string) => row.includes("◇ lens"));
    f.footer().handleMouse({ type: "click", button: "left", x: narrow[y].indexOf("◇ lens"), y });
    expect(click).toHaveBeenCalledTimes(2);
  } finally { registry.widgets.delete("pi-lens"); h.dispose(); }
});
