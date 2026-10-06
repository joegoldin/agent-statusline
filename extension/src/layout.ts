import { activityRows } from "./activity";
import { paint, type ThemeLike } from "./intents";
import type { Snapshot, Span } from "./snapshot";
import { truncateEnd, visibleWidth } from "./width";

// A port of internal/layout/layout.go, plus the line budget main.go applies on
// top of it. The Go side stays the authority on *what* each widget says; this
// file decides how much of it fits, which is a question only the renderer can
// answer because only the renderer knows the width pi pushed in.

interface Seg {
  name: string;
  text: string;
  flex: boolean;
  drop: number;
}

/**
 * Paint the named widgets into segments. `width` of 0 means "no budget": no
 * dropping and no compacting, matching Go's layout.Options{Width: 0}, which is
 * how the natural-width pass measures whether both rows fit on one line.
 */
function segments(names: string[], snap: Snapshot, width: number, theme: ThemeLike): Seg[] {
  const cfg = snap.config;
  const hide = new Set(cfg.hide ?? []);
  const dropIndex = new Map((cfg.dropPriority ?? []).map((n, i) => [n, i] as const));
  const fallbackDrop = (cfg.dropPriority ?? []).length + 1;
  const compact = width > 0 && width < cfg.compactWidth;

  const out: Seg[] = [];
  for (const name of names) {
    if (hide.has(name)) continue;
    if (name === cfg.flexName) {
      out.push({ name, text: "", flex: true, drop: 0 });
      continue;
    }
    const w = snap.widgets?.[name];
    if (!w || w.visible === false) continue;
    const spans: Span[] | undefined = compact ? (w.compact ?? w.spans) : w.spans;
    if (!spans || spans.length === 0) continue;
    const text = paint(spans, theme, cfg);
    if (text === "") continue;
    out.push({ name, text, flex: false, drop: dropIndex.get(name) ?? fallbackDrop });
  }
  return out;
}

/**
 * Emit segments with separators, expanding flex spacers to fill the remaining
 * width. A separator goes only between two adjacent non-flex segments — a flex
 * spacer is itself the gap, and doubling it up would shift the right-hand
 * group by three cells.
 */
function joinSegments(segs: Seg[], separator: string, width: number): string {
  let fixed = 0;
  let flexCount = 0;
  for (const s of segs) {
    if (s.flex) {
      flexCount++;
      continue;
    }
    fixed += visibleWidth(s.text);
  }
  let adjSeparators = 0;
  for (let i = 1; i < segs.length; i++) {
    if (!segs[i]!.flex && !segs[i - 1]!.flex) adjSeparators++;
  }
  const totalFixed = fixed + adjSeparators * visibleWidth(separator);
  const flexBudget = Math.max(0, width - totalFixed);
  const perFlex = flexCount > 0 ? Math.floor(flexBudget / flexCount) : 0;
  let flexRemainder = flexCount > 0 ? flexBudget - perFlex * flexCount : 0;

  let out = "";
  let prevWasVisible = false;
  let prevWasFlex = false;
  for (const s of segs) {
    if (s.flex) {
      let n = perFlex;
      if (flexRemainder > 0) {
        n++;
        flexRemainder--;
      }
      out += " ".repeat(Math.max(0, n));
      prevWasFlex = true;
      prevWasVisible = true;
      continue;
    }
    if (prevWasVisible && !prevWasFlex) out += separator;
    out += s.text;
    prevWasVisible = true;
    prevWasFlex = false;
  }
  return out;
}

/** The dropable segment with the lowest drop priority, or -1. */
function lowestPriorityIdx(segs: Seg[]): number {
  let best = -1;
  for (let i = 0; i < segs.length; i++) {
    if (segs[i]!.flex) continue;
    if (best === -1 || segs[i]!.drop < segs[best]!.drop) best = i;
  }
  return best;
}

/** Render one row, dropping the lowest-priority widget until it fits. */
export function composeRow(
  names: string[],
  snap: Snapshot,
  width: number,
  theme: ThemeLike,
): string {
  const sep = snap.config.separator;
  let segs = segments(names, snap, width, theme);
  for (;;) {
    const body = joinSegments(segs, sep, width);
    if (width <= 0 || visibleWidth(body) <= width) return body;
    const idx = lowestPriorityIdx(segs);
    if (idx === -1) return truncateEnd(body, width);
    segs = segs.filter((_, i) => i !== idx);
  }
}

/**
 * Pack a row's visible segments across as many lines as needed, so nothing is
 * dropped. Flex spacers are ignored: right-alignment is meaningless once
 * content wraps.
 */
export function wrapRow(names: string[], snap: Snapshot, width: number, theme: ThemeLike): string[] {
  const sep = snap.config.separator;
  const texts = segments(names, snap, width, theme)
    .filter((s) => !s.flex)
    .map((s) => s.text);
  if (texts.length === 0) return [];

  const sepW = visibleWidth(sep);
  const lines: string[] = [];
  let cur = "";
  let curW = 0;
  for (const t of texts) {
    const tw = visibleWidth(t);
    if (cur === "") {
      cur = t;
      curW = tw;
    } else if (width > 0 && curW + sepW + tw > width) {
      lines.push(cur);
      cur = t;
      curW = tw;
    } else {
      cur += sep + t;
      curW += sepW + tw;
    }
  }
  if (cur !== "") lines.push(cur);
  return width > 0 ? lines.map((l) => (visibleWidth(l) > width ? truncateEnd(l, width) : l)) : lines;
}

/**
 * Every line the statusline draws, already fitted to `width`.
 *
 * Every dashboard widget, rows 1 to 4 in order, is packed into as few lines as
 * fit: a line breaks only when the next widget would not fit on it, so a widget
 * left over from row 1 shares its line with row 2's rather than taking a line
 * of its own. Nothing is truncated. A single activity line joins the last
 * dashboard line when it fits there too; otherwise activity fills what is left
 * of the line budget, as before.
 *
 * This packs tighter than main.go, which keeps rows 1 and 2 as separate wrapped
 * blocks unless both fit on one line. pi's footer sits under the prompt and
 * every line it takes is one the transcript loses; Claude Code's statusline
 * keeps main.go's layout, which its goldens pin.
 */
export function renderRows(
  snap: Snapshot,
  width: number,
  theme: ThemeLike,
  now: number,
): string[] {
  const cfg = snap.config;
  const sep = cfg.separator;
  const maxLines = cfg.maxLines > 0 ? cfg.maxLines : 6;

  const names = [...(cfg.row1 ?? []), ...(cfg.row2 ?? []), ...(cfg.row3 ?? []), ...(cfg.row4 ?? [])];
  const dashboard = wrapRow(names, snap, width, theme)
    .filter((l) => l.trim().length > 0)
    .slice(0, maxLines);

  const budget = Math.min(maxLines - dashboard.length, cfg.activityRows ?? 0);
  // Asked for one extra row, in case the first activity line moves up.
  const activity = budget > 0 ? activityRows(snap, width, theme, now, budget + 1) : [];
  const last = dashboard.length - 1;
  if (activity.length > 0 && last >= 0) {
    const joined = dashboard[last] + sep + activity[0];
    if (width <= 0 || visibleWidth(joined) <= width) {
      dashboard[last] = joined;
      activity.shift();
    }
  }

  const padding = cfg.padding > 0 ? cfg.padding : 0;
  const pad = padding > 0 ? " ".repeat(padding) : "";
  return [...dashboard, ...activity.slice(0, Math.max(0, budget))]
    .map((l) => (pad ? pad + truncateEnd(l, width - padding) : l))
    .filter((l) => l.trim().length > 0);
}
