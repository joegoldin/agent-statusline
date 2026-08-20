import { paint, type ThemeLike } from "./intents";
import type { ActivityItem, Snapshot, Span } from "./snapshot";
import { truncateEnd, truncateMiddle, visibleWidth } from "./width";

// Glyphs and frames copied out of internal/widgets/activity.go rather than
// retyped, so the two renderers draw the same shapes.
const DONE_GLYPH = "✓";
const TODO_GLYPH = "▸";
const ALL_DONE_GLYPH = "✓";
const WAITING_GLYPH = ""; // nf-fa-hourglass-half
const ITEM_SEPARATOR = "  ·  ";

/**
 * A play button whose fill sweeps across two arrows and drains back out. Far
 * more legible at a 1 Hz repaint than a braille spinner, and — unlike under
 * Claude Code, where the frame only advanced when the harness re-invoked the
 * binary — this one now actually animates, because the component ticks
 * tui.requestRender() on its own timer.
 */
export const SPINNER_FRAMES = ["▷▷", "▶▷", "▶▶", "▷▶"];

const SPINNER_CELLS = SPINNER_FRAMES.reduce((w, f) => Math.max(w, visibleWidth(f)), 0);

/**
 * The animation frame for a moment in time, right-padded to a constant cell
 * width. The frames can differ in raw width, and the per-tool truncation
 * budget is computed against the glyph, so an unpadded spinner would shift
 * every command's middle-ellipsis cut by a cell each second.
 */
export function spinnerFrame(now: number): string {
  const idx = ((Math.floor(now / 1000) % SPINNER_FRAMES.length) + SPINNER_FRAMES.length) %
    SPINNER_FRAMES.length;
  const frame = SPINNER_FRAMES[idx]!;
  const pad = SPINNER_CELLS - visibleWidth(frame);
  return pad > 0 ? frame + " ".repeat(pad) : frame;
}

/** A port of Go's formatDuration: "45s", "5m30s", "1h12m", "2d3h". */
export function formatDuration(ms: number): string {
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m${s % 60}s`;
  if (s < 86400) return `${Math.floor(s / 3600)}h${Math.floor((s % 3600) / 60)}m`;
  return `${Math.floor(s / 86400)}d${Math.floor((s % 86400) / 3600)}h`;
}

function clipForAgent(s: string, max: number): string {
  const chars = Array.from(s);
  return chars.length <= max ? s : chars.slice(0, max - 1).join("") + "…";
}

function text(intent: Span["intent"], value: string): Span {
  return { kind: "text", text: value, intent };
}

/** Elapsed for one tool, measured against the render clock, not the snapshot. */
function toolElapsed(it: ActivityItem, now: number): number {
  const start = it.startedAtMs || it.emittedAtMs || 0;
  if (it.state === "waiting") return it.emittedAtMs ? now - it.emittedAtMs : 0;
  if (it.state === "done") return it.endedAtMs && start ? it.endedAtMs - start : 0;
  return start ? now - start : 0;
}

function elapsedText(ms: number): string {
  return ms >= 1000 ? `(${formatDuration(ms)}) ` : "";
}

/**
 * The running / just-finished tools row.
 *
 * Grace filtering happens here rather than in Go, against the render clock, so
 * a finished tool drops off on time instead of at the next binary invocation.
 * Capping and truncation happen here too, because both depend on the width pi
 * pushes in and the snapshot has no idea what that is.
 */
function toolsRow(snap: Snapshot, width: number, now: number): Span[] | undefined {
  const graces = snap.activity.graces;
  const items = (snap.activity.tools ?? []).filter((it) => {
    if (it.state !== "done") return true;
    return !!it.endedAtMs && now - it.endedAtMs <= graces.toolCompleteMs;
  });
  if (items.length === 0) return undefined;

  // Most-recent first: running and waiting tools count as "now" so they stay
  // on top; finished commands sort by completion time.
  const recency = (it: ActivityItem) => (it.state === "done" ? (it.endedAtMs ?? 0) : now);
  const sorted = [...items].sort((a, b) => recency(b) - recency(a));

  const w = width > 0 ? width : 80;
  const shown = sorted.slice(0, w >= 120 ? 3 : 2);
  const n = shown.length;
  const sepW = visibleWidth(ITEM_SEPARATOR);
  const perTool = Math.floor((w - (n - 1) * sepW) / n);

  const spans: Span[] = [];
  shown.forEach((it, i) => {
    if (i > 0) spans.push(text("muted", ITEM_SEPARATOR));
    const glyph =
      it.state === "waiting" ? WAITING_GLYPH : it.state === "done" ? DONE_GLYPH : spinnerFrame(now);
    const intent = it.state === "waiting" ? "dim" : it.state === "done" ? "ok" : "warn";
    const elapsed = elapsedText(toolElapsed(it, now));
    let label = it.name;
    if (it.target) label += `: ${it.target}`;
    // Budget against the prefix's actual cell width: the done tick and the
    // waiting hourglass are not the same width as the spinner, and a fixed
    // assumption overflows into a spurious trailing ellipsis.
    const budget = Math.max(1, perTool - visibleWidth(`${glyph} ${elapsed}`));
    spans.push(
      text(intent, `${glyph} `),
      text("dim", elapsed),
      text(intent, truncateMiddle(label, budget)),
    );
  });
  return spans;
}

/** The subagent row. */
function agentsRow(snap: Snapshot, now: number): Span[] | undefined {
  const graces = snap.activity.graces;
  const all = snap.activity.agents ?? [];
  const running = all.filter((a) => !a.endedAtMs && now - a.startedAtMs <= graces.agentRunningStaleMs);
  const completed = all
    .filter((a) => !!a.endedAtMs && now - a.endedAtMs! <= graces.agentCompleteMs)
    .slice(0, 2);
  const pick = [...running, ...completed].slice(0, 3);
  if (pick.length === 0) return undefined;

  const spans: Span[] = [];
  pick.forEach((a, i) => {
    if (i > 0) spans.push(text("muted", ITEM_SEPARATOR));
    const done = !!a.endedAtMs;
    const elapsed = elapsedText(done ? a.endedAtMs! - a.startedAtMs : now - a.startedAtMs);
    spans.push(
      text(done ? "ok" : "warn", `${done ? DONE_GLYPH : spinnerFrame(now)} `),
      text("dim", elapsed),
      text("meta", a.name),
    );
    if (a.model) spans.push(text("dim", ` [${a.model}]`));
    if (a.description) spans.push(text("dim", `: ${clipForAgent(a.description, 40)}`));
  });
  return spans;
}

/** The todo row: the in-progress item, or a brief all-complete celebration. */
function todosRow(snap: Snapshot, now: number): Span[] | undefined {
  const t = snap.activity.todos;
  if (!t || t.total <= 0) return undefined;
  if (t.subject) {
    return [
      text("accent", `${TODO_GLYPH} ${clipForAgent(t.subject, 50)} `),
      text("dim", `(${t.done}/${t.total})`),
    ];
  }
  if (!t.allComplete) return undefined;
  if (!t.timestampMs || now - t.timestampMs > snap.activity.graces.todoCompleteMs) return undefined;
  return [
    text("ok", `${ALL_DONE_GLYPH} all todos complete `),
    text("dim", `(${t.done}/${t.total})`),
  ];
}

/**
 * The activity stack, at most `budget` rows, each already fitted to `width`.
 *
 * Everything time-dependent — the spinner frame, every elapsed counter, every
 * grace window — is evaluated here against `now`, which is the component's
 * clock rather than the snapshot's. That is what lets a 1 Hz repaint keep the
 * rows alive without respawning the binary.
 */
export function activityRows(
  snap: Snapshot,
  width: number,
  theme: ThemeLike,
  now: number,
  budget: number,
): string[] {
  if (budget <= 0) return [];
  const rows: string[] = [];
  for (const build of [
    () => toolsRow(snap, width, now),
    () => agentsRow(snap, now),
    () => todosRow(snap, now),
  ]) {
    if (rows.length >= budget) break;
    let spans: Span[] | undefined;
    try {
      spans = build();
    } catch {
      spans = undefined;
    }
    if (!spans || spans.length === 0) continue;
    const line = paint(spans, theme, snap.config);
    rows.push(width > 0 ? truncateEnd(line, width) : line);
  }
  return rows;
}
