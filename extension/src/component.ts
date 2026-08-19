import type { ThemeLike } from "./intents";
import { renderRows } from "./layout";
import type { Snapshot } from "./snapshot";
import { truncateEnd } from "./width";

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
  now?: () => number;
}

export interface StatuslineHandle {
  setSnapshot(s: Snapshot): void;
  dispose(): void;
}

const DEFAULT_INTERVAL_MS = 1000;

/**
 * Install the native statusline.
 *
 * setFooter returns [] purely to blank pi's built-in footer and to capture the
 * tui/theme/footerData handles — footerData is the only route to the git branch
 * and to other extensions' setStatus text. The drawing all happens in ONE
 * belowEditor widget, because the dashboard and activity rows share a single
 * line budget (activityBudget = maxLines - dashboard.length) and two independent
 * components cannot see each other's line count.
 *
 * Nothing is ever passed to setStatus except undefined: that sink collapses
 * newlines and runs of spaces (footer.ts:13-19), which destroys both our row
 * structure and our flex padding. src/pi-contract.test.ts enforces this.
 */
export function installStatusline(ctx: PiUIContext, deps: InstallDeps): StatuslineHandle {
  const noop: StatuslineHandle = {
    setSnapshot() {},
    dispose() {},
  };
  // setWidget and setFooter are stubbed headless (runner.ts) and setFooter is
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

  const clearTick = () => {
    if (tickHandle) clearInterval(tickHandle);
    tickHandle = undefined;
    tickMs = 0;
  };

  /**
   * Re-render other extensions' setStatus lines. We took the footer, so pi no
   * longer draws them; dropping them would make us a bad neighbour. Split on
   * [\r\n] to recover the multi-row capability setStatus denies, and dim only
   * lines that brought no colour of their own.
   */
  const foreignStatusRows = (width: number, theme: ThemeLike): string[] => {
    const out: string[] = [];
    let entries: Array<[string, string]>;
    try {
      entries = Array.from(statuses().entries()).sort(([a], [b]) => a.localeCompare(b));
    } catch {
      return out;
    }
    for (const [key, text] of entries) {
      if (key === WIDGET_KEY || !text) continue;
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
    return out;
  };

  // Clear any line a previous version of this extension left in the footer's
  // status map, so the row we replaced cannot reappear underneath us.
  ctx.ui.setStatus(WIDGET_KEY, undefined);

  ctx.ui.setFooter((tui: PiTUI, _theme: ThemeLike, footerData: PiFooterData) => {
    tuiRef = tui;
    statuses = () => footerData.getExtensionStatuses();
    unsubBranch = footerData.onBranchChange(() => {
      // pi already watches HEAD and the reftable with a 500 ms debounce, so a
      // branch switch refreshes the snapshot for free.
      deps.onDataStale?.();
      tui.requestRender();
    });
    armTick();
    return {
      invalidate() {},
      dispose() {
        unsubBranch?.();
        unsubBranch = undefined;
      },
      render(): string[] {
        return [];
      },
    };
  });

  ctx.ui.setWidget(
    WIDGET_KEY,
    (tui: PiTUI, theme: ThemeLike) => {
      tuiRef = tui;
      armTick();
      let lastGood: string[] = [];
      return {
        // pi calls invalidate() on theme change; nothing is memoised, so there
        // is nothing to drop — but a repaint is still wanted.
        invalidate() {
          tui.requestRender();
        },
        dispose() {
          clearTick();
        },
        render(width: number): string[] {
          if (!snapshot) return [];
          try {
            lastGood = renderRows(snapshot, width, theme, now());
          } catch {
            // A statusline must never break the session: keep the last frame.
          }
          return [...lastGood, ...foreignStatusRows(width, theme)];
        },
      };
    },
    { placement: "belowEditor" },
  );

  return {
    setSnapshot(s: Snapshot) {
      snapshot = s;
      armTick();
      tuiRef?.requestRender();
    },
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
