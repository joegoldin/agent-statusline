// The wire format emitted by `agent-statusline --mode pi --emit json`.
// Every field name here matches a JSON tag in internal/emit/emit.go,
// internal/render/span.go and internal/widgets/activity_snapshot.go. Changing
// one without the other breaks the pi path silently, which is why
// internal/e2e/testdata/pi-full.json.golden exists on the Go side and why the
// fixture under testdata/ here is the binary's real output rather than a
// hand-written one.

export type Intent =
  | "text"
  | "dim"
  | "muted"
  | "accent"
  | "meta"
  | "path"
  | "ok"
  | "warn"
  | "caution"
  | "danger";

export interface Span {
  kind: "text" | "bar";
  text?: string;
  intent?: Intent;
  link?: string;
  /** bar only: fraction in [0,1], NOT a percentage */
  fill?: number;
  cells?: number;
  style?: "braille" | "block" | "line";
}

export interface WidgetSnapshot {
  visible: boolean;
  spans?: Span[];
  /** Present only when the widget's narrow form differs from its wide one. */
  compact?: Span[];
}

export interface SnapshotConfig {
  barWidth: number;
  compactWidth: number;
  activityRows: number;
  hideWhenIdle: boolean;
  padding: number;
  refreshIntervalMs: number;
  maxLines: number;
  separator: string;
  flexName: string;
  row1: string[];
  row2: string[];
  row3: string[];
  row4: string[];
  hide: string[];
  dropPriority: string[];
}

export interface ActivityItem {
  id: string;
  name: string;
  target?: string;
  state: "running" | "waiting" | "done";
  emittedAtMs?: number;
  startedAtMs?: number;
  endedAtMs?: number;
}

export interface AgentItem {
  name: string;
  model?: string;
  description?: string;
  startedAtMs: number;
  endedAtMs?: number;
}

export interface TodoItem {
  subject?: string;
  done: number;
  total: number;
  allComplete: boolean;
  timestampMs?: number;
}

export interface ActivityGraces {
  toolCompleteMs: number;
  agentCompleteMs: number;
  agentRunningStaleMs: number;
  todoCompleteMs: number;
}

export interface ActivitySnapshot {
  graces: ActivityGraces;
  tools: ActivityItem[] | null;
  agents: AgentItem[] | null;
  todos: TodoItem | null;
}

export interface Snapshot {
  schema: number;
  mode: string;
  asOfMs: number;
  config: SnapshotConfig;
  widgets: Record<string, WidgetSnapshot>;
  activity: ActivitySnapshot;
}

export const SUPPORTED_SCHEMA = 1;

/**
 * Parse a snapshot, refusing anything this renderer does not understand.
 *
 * A wrong statusline is worse than none, so a schema bump, malformed JSON or a
 * missing config block all return undefined and the component keeps drawing
 * the last good snapshot.
 */
export function parseSnapshot(raw: string): Snapshot | undefined {
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return undefined;
  }
  if (!parsed || typeof parsed !== "object") return undefined;
  const s = parsed as Partial<Snapshot>;
  if (s.schema !== SUPPORTED_SCHEMA) return undefined;
  if (!s.config || typeof s.config.separator !== "string") return undefined;
  if (!s.widgets || typeof s.widgets !== "object") return undefined;
  return s as Snapshot;
}
