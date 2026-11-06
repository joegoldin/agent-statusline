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
export function wrapRow(names: string[], snap: Snapshot, width: number, theme: ThemeLike, firstWidth = width, tail: string[] = []): string[] {
  const sep = snap.config.separator;
  const texts = segments(names, snap, width, theme)
    .filter((s) => !s.flex)
    .map((s) => s.text).concat(tail);
  return wrapTexts(texts, sep, width, row => row === 0 ? firstWidth : width);
}

function wrapTexts(texts: string[], separator: string, width: number, rowWidth: (row: number) => number): string[] {
  const lines: string[] = [];
  const sepWidth = visibleWidth(separator);
  let current = "";
  let used = 0;
  for (const text of texts) {
    const cells = visibleWidth(text);
    if (current && width > 0 && used + sepWidth + cells > rowWidth(lines.length)) {
      lines.push(current);
      current = "";
      used = 0;
    }
    // A widget too wide beside a metric moves below it, rather than being
    // clipped into the reserved cells or painting over the right column.
    while (!current && width > 0 && cells > rowWidth(lines.length) && rowWidth(lines.length) < width) lines.push("");
    used += (current ? sepWidth : 0) + cells;
    current += (current ? separator : "") + text;
  }
  if (current) lines.push(current);
  return width > 0 ? lines.map((line, row) => truncateEnd(line, rowWidth(row))) : lines;
}

/**
 * Every line the statusline draws, already fitted to `width`.
 *
 * Every dashboard widget, rows 1 to 4 in order, is packed into as few lines as
 * fit: a line breaks only when the next widget would not fit on it, so a widget
 * left over from row 1 shares its line with row 2's rather than taking a line
 * of its own. Speed and context reserve separate right-hand rows before the
 * left flow is wrapped. Compact terminals combine everything on the left.
 * Activity uses any remaining room within the line budget.
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
  right = "",
  inline: string[] = [],
): string[] {
  const cfg = snap.config;
  const sep = cfg.separator;
  const maxLines = cfg.maxLines > 0 ? cfg.maxLines : 6;

  const names = [...(cfg.row1 ?? []), ...(cfg.row2 ?? []), ...(cfg.row3 ?? []), ...(cfg.row4 ?? [])];
  const cost = snap.widgets.cost;
  const model = snap.widgets.model;
  // Membership is part of the model identity; metered API costs remain an
  // independent widget with the usual separator. Keep each span's own colour.
  if (names.includes("model") && names.includes("cost") && !cfg.hide.includes("cost") && model?.visible && cost?.visible && cost.spans?.length === 1 && cost.spans[0].text === "") {
    const suffix: Span[] = [{ kind: "text", text: " " }, ...cost.spans];
    snap = { ...snap, widgets: { ...snap.widgets,
      model: { ...model, spans: [...(model.spans ?? []), ...suffix], compact: [...(model.compact ?? model.spans ?? []), ...suffix] },
      cost: { ...cost, visible: false },
    } };
  }
  const contextParts = segments(["tokens", "context"].filter(name => names.includes(name)), snap, width, theme).map(segment => segment.text);
  const context = contextParts.join(sep);
  const padding = Math.max(0, width > 0 ? Math.min(cfg.padding, width - 1) : cfg.padding);
  const innerWidth = width > 0 ? width - padding : 0;
  const metrics = [right, context].filter(Boolean).slice(0, maxLines);
  // Use the existing compact breakpoint, and fall back sooner if a metric
  // would leave fewer than twelve cells for the left-hand widgets.
  const dock = width >= cfg.compactWidth && width > 0 && metrics.every(text => visibleWidth(text) + 14 <= innerWidth);
  const pinned = dock ? metrics : [];
  const rowWidth = (row: number) => innerWidth - (pinned[row] ? visibleWidth(pinned[row]) + 2 : 0);
  // Voice is already visible in the prompt; its backend mode is not a status.
  const flow = names.filter(name => name !== "context" && name !== "tokens" && name !== "voice");
  const texts = segments(flow, snap, width, theme).filter(segment => !segment.flex).map(segment => segment.text).concat(inline);
  if (!dock) {
    if (right) texts.push(right);
    if (context) texts.push(...(innerWidth > 0 && visibleWidth(context) > innerWidth ? contextParts : [context]));
  }
  const dashboard = wrapTexts(texts, sep, innerWidth, rowWidth)
    .slice(0, inline.length ? undefined : maxLines);

  const budget = Math.min(maxLines - Math.max(dashboard.length, pinned.length), cfg.activityRows ?? 0);
  const activity = budget > 0 ? activityRows({ ...snap, activity: { ...snap.activity, tools: [] } }, innerWidth, theme, now, budget + 1) : [];
  const last = dashboard.length - 1;
  if (activity.length > 0 && last >= 0) {
    const joined = dashboard[last] + (dashboard[last] ? sep : "") + activity[0];
    if (innerWidth <= 0 || visibleWidth(joined) <= rowWidth(last)) {
      dashboard[last] = joined;
      activity.shift();
    }
  }

  const left = [...dashboard];
  while (left.length < pinned.length) left.push("");
  left.push(...activity.slice(0, Math.max(0, budget)));
  return Array.from({ length: Math.max(left.length, pinned.length) }, (_, row) => {
    const body = " ".repeat(padding) + (left[row] ?? "");
    const metric = pinned[row] ?? "";
    return metric ? body + " ".repeat(Math.max(0, width - visibleWidth(body) - visibleWidth(metric))) + metric : body;
  }).filter(line => line.trim().length > 0);
}
