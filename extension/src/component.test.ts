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
  it("registers one belowEditor widget and blanks the footer", () => {
    const f = fakeUI();
    installStatusline(f.ctx as any, {});
    expect(f.widgets.has(WIDGET_KEY)).toBe(true);
    expect(f.widgets.size).toBe(1);
    expect(f.widgets.get(WIDGET_KEY)!.options.placement).toBe("belowEditor");
    expect(f.footer()!.render(120)).toEqual([]);
  });

  it("never routes rows through setStatus", () => {
    const f = fakeUI();
    const h = installStatusline(f.ctx as any, {});
    h.setSnapshot(snap);
    f.widgets.get(WIDGET_KEY)!.component.render(120);
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
    expect(f.widgets.get(WIDGET_KEY)!.component.render(120)).toEqual([]);
  });

  it("renders rows once a snapshot is set", () => {
    const f = fakeUI();
    const h = installStatusline(f.ctx as any, {});
    h.setSnapshot(snap);
    expect(f.widgets.get(WIDGET_KEY)!.component.render(120).length).toBeGreaterThan(0);
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
    const rows = f.widgets.get(WIDGET_KEY)!.component.render(200).join("\n");
    expect(rows).toContain("other line one");
    expect(rows).toContain("other line two");
    expect(rows).not.toContain("stale");
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

  it("removes the widget and restores the footer on dispose", () => {
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
    const good = f.widgets.get(WIDGET_KEY)!.component.render(120);
    // A snapshot parseSnapshot would have refused. The component is the last
    // line of defence: a statusline must never take the session down with it.
    h.setSnapshot({ ...snap, config: null as any });
    expect(f.widgets.get(WIDGET_KEY)!.component.render(120)).toEqual(good);
  });

  it("repaints when the theme changes, via invalidate", () => {
    const f = fakeUI();
    const h = installStatusline(f.ctx as any, {});
    h.setSnapshot(snap);
    f.requestRender.mockClear();
    f.widgets.get(WIDGET_KEY)!.component.invalidate();
    expect(f.requestRender).toHaveBeenCalled();
  });
});
