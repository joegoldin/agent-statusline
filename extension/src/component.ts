import { paint, type ThemeLike } from "./intents";
import { renderRows } from "./layout";
import type { Snapshot, Span } from "./snapshot";
import { truncateEnd, visibleWidth } from "./width";

export const WIDGET_KEY = "agent-statusline";

/** The pi surfaces this module touches, declared structurally (no imports). */
export interface PiTUI {
  requestRender(force?: boolean): void;
}

export interface PiFooterData {
  getExtensionStatuses(): ReadonlyMap<string, string>;
  onBranchChange(cb: () => void): () => void;
}

export interface PiUIContext {
  mode: string;
  hasUI: boolean;
  ui: {
    setStatus(key: string, text: string | undefined): void;
    setWidget(
      key: string,
      content: unknown,
      options?: { placement?: "aboveEditor" | "belowEditor" },
    ): void;
    setFooter(factory: unknown): void;
    theme?: ThemeLike;
  };
}

export interface InstallDeps {
  /** Called when something happened that the snapshot cannot know about. */
  onDataStale?: () => void;
  /**
   * Other extensions' status keys whose text the snapshot already draws as
   * widgets, asked on every frame because the answer follows the session.
   */
  absorbedStatusKeys?: () => ReadonlySet<string>;
  now?: () => number;
  performance?: () => Span[];
}

export interface StatuslineHandle {
  requestRender(): void;
  setSnapshot(s: Snapshot): void;
  /**
   * Another extension's setStatus text. Taking the footer made this component
   * the only reader of pi's status map, so a payload field built from one of
   * those lines has to come through here.
   */
  extensionStatus(key: string): string | undefined;
  dispose(): void;
}

const DEFAULT_INTERVAL_MS = 1000;
const FAST_STATUS_KEY = "pi-fast";

interface InlineWidget {
  render(width: number, theme: ThemeLike): string;
  handleMouse(event: { type: string; button: string; x: number; y: number }): unknown;
}
interface InlineRegistry {
  widgets: Map<string, InlineWidget>;
  consumers: Set<object>;
}
// Shared with Pi's Lens adapter; either extension can load first, and Lens
// keeps its standalone widget when no dashboard consumes its compact chip.
const INLINE_WIDGETS = Symbol.for("pi.inline-status-widgets.v1");
function inlineWidgets(): InlineRegistry {
  const shared = globalThis as typeof globalThis & { [INLINE_WIDGETS]?: InlineRegistry };
  return shared[INLINE_WIDGETS] ??= { widgets: new Map(), consumers: new Set() };
}


function withFastMode(snapshot: Snapshot, active: boolean): Snapshot {
  const model = snapshot.widgets.model;
  if (!active || !model?.visible) return snapshot;
  // Model spans already include effort. Decorate at render time so a toggle
  // never waits for the next binary refresh or modifies the cached snapshot.
  const suffix: Span = { kind: "text", intent: "warn", text: " fast" };
  return {
    ...snapshot,
    widgets: {
      ...snapshot.widgets,
      model: {
        ...model,
        spans: [...(model.spans ?? []), suffix],
        compact: model.compact ? [...model.compact, suffix] : undefined,
      },
    },
  };
}

/**
 * Install the native statusline.
 *
 * The native footer is always below extension widgets, regardless of loading
 * order. Keep the dashboard and metrics together there so Lens and Jobs cannot
 * end up beneath them, and all statusline rows share the same line budget.
 *
 * Nothing is ever passed to setStatus except undefined: that sink collapses
 * newlines and runs of spaces (footer.ts:13-19), which destroys both our row
 * structure and our flex padding. src/pi-contract.test.ts enforces this.
 */
export function installStatusline(ctx: PiUIContext, deps: InstallDeps): StatuslineHandle {
  const noop: StatuslineHandle = {
    requestRender() {},
    setSnapshot() {},
    extensionStatus: () => undefined,
    dispose() {},
  };
  // setFooter is stubbed headless (runner.ts) and is
  // absent over RPC, so every UI call is gated on being in a real TUI.
  if (ctx?.mode !== "tui" || !ctx.hasUI || !ctx.ui) return noop;

  const now = deps.now ?? (() => Date.now());
  let snapshot: Snapshot | undefined;
  let statuses: () => ReadonlyMap<string, string> = () => new Map();
  let tickHandle: ReturnType<typeof setInterval> | undefined;
  let tickMs = 0;
  let tuiRef: PiTUI | undefined;
  let unsubBranch: (() => void) | undefined;
  let disposed = false;

  const armTick = () => {
    const want = snapshot?.config?.refreshIntervalMs || DEFAULT_INTERVAL_MS;
    if (!tuiRef || disposed || want === tickMs) return;
    if (tickHandle) clearInterval(tickHandle);
    tickMs = want;
    // requestRender coalesces on process.nextTick and throttles to 16 ms
    // (pi-tui/src/tui.ts), so a 1 Hz call costs nothing. This is what unfreezes
    // the spinner and the elapsed clocks between events.
    tickHandle = setInterval(() => tuiRef?.requestRender(), tickMs);
    (tickHandle as { unref?: () => void }).unref?.();
  };

  const extensionStatus = (key: string): string | undefined => {
    try {
      const text = statuses().get(key);
      return typeof text === "string" && text !== "" ? text : undefined;
    } catch {
      return undefined;
    }
  };

  const clearTick = () => {
    if (tickHandle) clearInterval(tickHandle);
    tickHandle = undefined;
    tickMs = 0;
  };

  /**
   * Re-render other extensions' setStatus lines. We took the footer, so pi no
   * longer draws them; dropping them would make us a bad neighbour. Split on
   * [\r\n] to recover the multi-row capability setStatus denies, and dim only
   * lines that brought no colour of their own. Lines a widget already draws
   * are skipped, or the footer would say the same thing twice.
   */
  const inline = inlineWidgets();
  const consumer = {};
  let inlineChips: Array<{ text: string; widget: InlineWidget }> = [];
  const foreignStatusRows = (width: number, theme: ThemeLike): string[] => {
    const out: string[] = [];
    let entries: Array<[string, string]>;
    let absorbed: ReadonlySet<string>;
    try {
      entries = Array.from(statuses().entries()).sort(([a], [b]) => a.localeCompare(b));
    } catch {
      entries = [];
    }
    try {
      absorbed = deps.absorbedStatusKeys?.() ?? new Set();
    } catch {
      // Showing a line twice beats losing every other extension's line.
      absorbed = new Set();
    }
    for (const [key, text] of entries) {
      if (key === WIDGET_KEY || key === FAST_STATUS_KEY || absorbed.has(key) || !text) continue;
      for (const line of String(text).split(/[\r\n]+/)) {
        if (!line.trim()) continue;
        const cut = width > 0 ? truncateEnd(line, width) : line;
        if (cut.includes("\x1b")) {
          out.push(cut);
          continue;
        }
        try {
          out.push(theme.fg("dim", cut));
        } catch {
          out.push(cut);
        }
      }
    }
    inlineChips = [];
    for (const widget of inline.widgets.values()) {
      try {
        const text = widget.render(width, theme);
        if (text) {
          out.push(text);
          inlineChips.push({ text, widget });
        }
      } catch {
        // A broken extension must not hide the other dashboard chips.
      }
    }
    return out;
  };

  // Clear any line a previous version of this extension left in the footer's
  // status map, so the row we replaced cannot reappear underneath us.
  ctx.ui.setStatus(WIDGET_KEY, undefined);
  ctx.ui.setWidget(WIDGET_KEY, undefined);

  ctx.ui.setFooter((tui: PiTUI, theme: ThemeLike, footerData: PiFooterData) => {
    tuiRef = tui;
    statuses = () => footerData.getExtensionStatuses();
    unsubBranch = footerData.onBranchChange(() => {
      deps.onDataStale?.();
      tui.requestRender();
    });
    inline.consumers.add(consumer);
    armTick();
    let lastGood: string[] = [];
    return {
      invalidate() { tui.requestRender(); },
      dispose() {
        inline.consumers.delete(consumer);
        clearTick();
        unsubBranch?.();
        unsubBranch = undefined;
      },
      handleMouse(event: { type: string; button: string; x: number; y: number }) {
        const row = lastGood[event.y];
        if (row === undefined) return;
        for (const { text, widget } of inlineChips) {
          const offset = row.indexOf(text);
          if (offset < 0) continue;
          const start = visibleWidth(row.slice(0, offset));
          if (event.x >= start && event.x < start + visibleWidth(text)) {
            return widget.handleMouse({ ...event, x: event.x - start, y: 0 });
          }
        }
      },
      render(width: number): string[] {
        if (!snapshot) return [];
        try {
          const badge = paint(deps.performance?.() ?? [], theme, snapshot.config);
          lastGood = renderRows(withFastMode(snapshot, extensionStatus(FAST_STATUS_KEY) === "fast"), width, theme, now(), badge, foreignStatusRows(width, theme));
        } catch {
          // A statusline must never break the session: keep the last frame.
        }
        return lastGood;
      },
    };
  });

  return {
    requestRender() { tuiRef?.requestRender(); },
    setSnapshot(s: Snapshot) {
      snapshot = s;
      armTick();
      tuiRef?.requestRender();
    },
    extensionStatus,
    dispose() {
      disposed = true;
      clearTick();
      // Both calls trigger the components' own dispose(), which is where the
      // branch subscription is released.
      ctx.ui.setWidget(WIDGET_KEY, undefined);
      ctx.ui.setFooter(undefined);
      ctx.ui.setStatus(WIDGET_KEY, undefined);
    },
  };
}
