// agent-statusline pi extension.
//
// Deliberately thin: it serialises only what pi natively exposes and lets the
// Go binary derive everything else (percentages, workspace fields, cost
// gating). Translation logic lives in Go because that is where the golden
// tests are.
//
// The structural interfaces below mirror the real pi type definitions shipped
// in @earendil-works/pi-coding-agent (core/extensions/types.d.ts) and
// @earendil-works/pi-ai (dist/types.d.ts). They are duplicated rather than
// imported so the extension file stays dependency-free: pi copies a single
// .ts file out of the Nix store with no node_modules beside it.

import { spawn } from "node:child_process";
import { readFileSync } from "node:fs";

import { installStatusline } from "./src/component";
import { parseSnapshot } from "./src/snapshot";

// ---------------------------------------------------------------------------
// The pi API surface we touch, mirrored from the real .d.ts files.
// ---------------------------------------------------------------------------

/** pi-ai `Usage`. Note `cacheWrite` (not `cacheCreation`) and the nested cost. */
export interface PiUsage {
  input: number;
  output: number;
  cacheRead: number;
  cacheWrite: number;
  totalTokens?: number;
  cost?: { input: number; output: number; cacheRead: number; cacheWrite: number; total: number };
}

/**
 * pi's `ContextUsage`. It is NOT a token breakdown: pi only reports an
 * estimated total, the window, and a precomputed percentage.
 */
export interface PiContextUsage {
  tokens: number | null;
  contextWindow: number;
  percent: number | null;
}

/** pi-ai `Model` — the display field is `name`, there is no `displayName`. */
export interface PiModel {
  id: string;
  name?: string;
  provider?: string;
  contextWindow?: number;
}

/** pi's shared event bus, the channel other extensions publish on. */
export interface PiEventBus {
  on(channel: string, handler: (data: unknown) => void): () => void;
}

/** The slice of `ExtensionContext` this extension reads. */
export interface PiExtensionContext {
  cwd?: string;
  model?: PiModel;
  thinkingLevel?: string;
  getContextUsage?: () => PiContextUsage | undefined;
  sessionManager?: {
    getCwd?: () => string;
    getSessionId?: () => string;
    getSessionFile?: () => string | undefined;
    getSessionName?: () => string | undefined;
    getBranch?: (fromId?: string) => unknown[];
  };
  ui?: { setStatus?: (key: string, text: string | undefined) => void };
}

// ---------------------------------------------------------------------------
// Wire format. Every field name matches an `input.PiStatus` JSON tag in Go.
// ---------------------------------------------------------------------------

export interface RateLimitWindow {
  used_percentage: number;
  resets_at: number;
}

export interface RateLimits {
  five_hour?: RateLimitWindow;
  seven_day?: RateLimitWindow;
}

export interface PiPayloadContext {
  window_size: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_creation_tokens: number;
}

export interface PiPayload {
  harness: "pi";
  cwd: string;
  session_id: string;
  session_name?: string;
  session_path?: string;
  project_dir?: string;
  model?: { id: string; display_name: string; provider?: string };
  thinking_level?: string;
  /**
   * The pi-automode extension's status text, verbatim. Parsed in Go, where the
   * widget can hide on a format change instead of rendering a wrong tally.
   */
  auto_mode?: string;
  /**
   * True when pi-usage reports a consumer subscription for the session, so
   * the Go side can hide a catalogue-priced cost nobody is paying.
   */
  subscription?: boolean;
  /** pi-lens's "pi-lens-lsp" status text, verbatim. Parsed in Go, like auto_mode. */
  lsp?: string;
  context?: PiPayloadContext;
  cost_usd?: number;
  duration_ms: number;
  api_duration_ms?: number;
  rate_limits?: RateLimits;
  version?: string;
}

/** Everything the extension accumulates across a session. */
export interface SessionState {
  sessionId: string;
  sessionName?: string;
  sessionPath?: string;
  projectDir?: string;
  startedAt: number;
  costUsd: number;
  apiDurationMs: number;
  /** Usage of the most recent assistant message — pi's only token breakdown. */
  lastUsage?: PiUsage;
  /** Limits off Anthropic's response headers. */
  rateLimits?: RateLimits;
  /** The model provider that was answering when `rateLimits` arrived. */
  rateLimitsProvider?: string;
  /** What pi-usage last reported, or undefined once it clears its status. */
  planUsage?: PlanUsage;
  version?: string;
  /** Last status text auto mode published, or undefined once it stops. */
  autoMode?: string;
  /** pi-lens's LSP status text as of the last refresh. */
  lsp?: string;
}

export function newSessionState(now = Date.now()): SessionState {
  return { sessionId: "", startedAt: now, costUsd: 0, apiDurationMs: 0 };
}

// ---------------------------------------------------------------------------
// Pure translation
// ---------------------------------------------------------------------------

function num(v: unknown): number {
  return typeof v === "number" && Number.isFinite(v) ? v : 0;
}

/**
 * buildPayload projects pi's live state onto the Go wire format.
 *
 * A statusline must never break the session, so every optional read is
 * best-effort: a throwing provider drops one widget, not the whole line.
 */
export function buildPayload(
  ctx: PiExtensionContext,
  state: SessionState,
  now = Date.now(),
): PiPayload {
  const payload: PiPayload = {
    harness: "pi",
    cwd: ctx?.cwd ?? "",
    session_id: state.sessionId,
    session_name: state.sessionName,
    session_path: state.sessionPath,
    project_dir: state.projectDir,
    duration_ms: Math.max(0, now - state.startedAt),
    api_duration_ms: state.apiDurationMs > 0 ? state.apiDurationMs : undefined,
    cost_usd: state.costUsd > 0 ? state.costUsd : undefined,
    rate_limits: activeRateLimits(state, ctx?.model?.provider).rateLimits,
    version: state.version,
    auto_mode: state.autoMode,
    // Only ever true or absent: absent is "not known to be a plan", which is
    // what keeps cost visible on an API key.
    subscription: state.planUsage?.subscription || undefined,
    lsp: state.lsp || undefined,
  };

  // pi-ai Model exposes `name`, not `displayName`. The provider rides along
  // because a model id alone does not identify a model to the cache-optimizer
  // sidecar the Go side reads, which keys on "provider/id".
  if (ctx?.model?.id) {
    payload.model = {
      id: ctx.model.id,
      display_name: ctx.model.name ?? ctx.model.id,
      provider: ctx.model.provider,
    };
  }
  if (ctx?.thinkingLevel) {
    payload.thinking_level = ctx.thinkingLevel;
  }

  let usage: PiContextUsage | undefined;
  try {
    usage = ctx?.getContextUsage?.();
  } catch {
    usage = undefined; // leave it unset; the Go side hides those widgets
  }

  const windowSize = num(usage?.contextWindow) || num(ctx?.model?.contextWindow);
  const last = state.lastUsage;
  if (last) {
    // pi's Usage names the cache-creation bucket `cacheWrite`.
    payload.context = {
      window_size: windowSize,
      input_tokens: num(last.input),
      output_tokens: num(last.output),
      cache_read_tokens: num(last.cacheRead),
      cache_creation_tokens: num(last.cacheWrite),
    };
  } else if (usage && typeof usage.tokens === "number") {
    // No assistant message seen yet (fresh or resumed session). pi's estimate
    // is a single total, so it lands entirely in input_tokens — that is what
    // Go's used-context accounting (input + both cache figures) then reports.
    payload.context = {
      window_size: windowSize,
      input_tokens: usage.tokens,
      output_tokens: 0,
      cache_read_tokens: 0,
      cache_creation_tokens: 0,
    };
  }

  return payload;
}

const HEADER_WINDOWS: ReadonlyArray<[keyof RateLimits, string]> = [
  ["five_hour", "5h"],
  ["seven_day", "7d"],
];

/**
 * rateLimitsFromHeaders reads Anthropic's unified rate-limit headers off an
 * `after_provider_response` event. Absent on Codex, OpenRouter and API-key
 * auth, which leave the widgets to pi-usage's report or hidden.
 */
export function rateLimitsFromHeaders(headers: Record<string, string> | undefined): RateLimits | undefined {
  if (!headers) return undefined;

  const lower: Record<string, string> = {};
  for (const [k, v] of Object.entries(headers)) {
    if (typeof v === "string") lower[k.toLowerCase()] = v;
  }

  let out: RateLimits | undefined;
  for (const [field, slug] of HEADER_WINDOWS) {
    const used = lower[`anthropic-ratelimit-unified-${slug}-used-percentage`];
    if (used === undefined) continue;
    const pct = Number(used);
    if (!Number.isFinite(pct)) continue;
    const reset = Number(lower[`anthropic-ratelimit-unified-${slug}-reset`] ?? 0);
    out ??= {};
    out[field] = { used_percentage: pct, resets_at: Number.isFinite(reset) ? reset : 0 };
  }
  return out;
}

/**
 * The channel auto mode republishes its status text on, from
 * extensions/auto-mode/permission-chain.ts. It only publishes when it has been
 * told not to draw its own status slot (PI_AUTOMODE_NO_STATUS_SLOT=1);
 * otherwise the text stays in pi's footer and this extension re-renders it
 * there like any other extension's status line.
 */
export const AUTO_MODE_STATUS_CHANNEL = "pi-automode:status";

/**
 * Validate one `{ text }` envelope off that channel.
 *
 * Envelope only: the text itself is parsed in Go, next to the widget that has
 * to hide when the format moves. An empty text is auto mode clearing its slot,
 * so it clears ours too rather than freezing the last tally on screen.
 */
export function autoModeText(data: unknown): string | undefined {
  if (!data || typeof data !== "object") return undefined;
  const text = (data as { text?: unknown }).text;
  return typeof text === "string" && text !== "" ? text : undefined;
}

/**
 * The channel pi-usage publishes its report on, alongside each update to its
 * "usage" status slot. `{ report: undefined }` means it cleared that slot or
 * has nothing ready, and clears ours.
 */
export const PI_USAGE_REPORT_CHANNEL = "pi-usage:report";

/** One window or balance from pi-usage's `UsageReport` (src/types.ts). */
export interface PiUsageBucket {
  id: string;
  label: string;
  groupId?: string;
  groupLabel?: string;
  modelKeys?: string[];
  used?: number;
  remaining?: number;
  limit?: number;
  unit: "percent" | "usd" | "currency" | "count";
  period?: string;
  windowMinutes?: number;
  /** Unix seconds, the same unit as the wire's `resets_at`. */
  resetsAt?: number;
}

/** pi-usage's `UsageReport`, mirrored from its src/types.ts. */
export interface PiUsageReport {
  providerId: string;
  providerName: string;
  capturedAt: number;
  source: string;
  semantics: { kind: "consumer-subscription" | "api-key" | "project"; label: string };
  accountLabel?: string;
  buckets: PiUsageBucket[];
  metrics: unknown[];
  notes?: string[];
}

/** What this extension keeps of a pi-usage report. */
export interface PlanUsage {
  subscription: boolean;
  rateLimits?: RateLimits;
}

/**
 * The ChatGPT app's own allowance. pi-usage reports it beside the plan
 * windows, but it meters the ChatGPT app, not pi, so it must not fill a slot.
 */
const CHATGPT_APP_GROUP = "chatgpt-app";

const FIVE_HOUR_MAX_MINUTES = 6 * 60;
const SEVEN_DAY_MIN_MINUTES = 6 * 24 * 60;
const SEVEN_DAY_MAX_MINUTES = 8 * 24 * 60;

/**
 * rateLimitsFromUsageBuckets maps pi-usage's percent windows onto the two
 * slots the usage widgets draw, by window length rather than by id: a window
 * of up to 6 h is the 5-hour slot, one of 6 to 8 days the 7-day slot. Every
 * provider names its windows differently, and the length is the one thing
 * they all report. A window of any other length, or with no length, has no
 * slot. When several windows land in one slot the fullest wins, because that
 * is the one that will stop the session first.
 */
export function rateLimitsFromUsageBuckets(buckets: readonly PiUsageBucket[] | undefined): RateLimits | undefined {
  if (!Array.isArray(buckets)) return undefined;
  let out: RateLimits | undefined;
  for (const bucket of buckets) {
    if (!bucket || typeof bucket !== "object") continue;
    if (bucket.unit !== "percent" || bucket.groupId === CHATGPT_APP_GROUP) continue;
    const used = bucket.used;
    const minutes = bucket.windowMinutes;
    if (typeof used !== "number" || !Number.isFinite(used)) continue;
    if (typeof minutes !== "number" || !Number.isFinite(minutes) || minutes <= 0) continue;

    let field: keyof RateLimits;
    if (minutes <= FIVE_HOUR_MAX_MINUTES) field = "five_hour";
    else if (minutes >= SEVEN_DAY_MIN_MINUTES && minutes <= SEVEN_DAY_MAX_MINUTES) field = "seven_day";
    else continue;

    if (out?.[field] && out[field]!.used_percentage >= used) continue;
    const reset = bucket.resetsAt;
    out ??= {};
    out[field] = {
      used_percentage: used,
      resets_at: typeof reset === "number" && Number.isFinite(reset) ? reset : 0,
    };
  }
  return out;
}

/**
 * Validate one `{ report }` envelope off pi-usage's channel. Anything that is
 * not recognisably a report clears the plan usage rather than keeping the last
 * one: a stale plan figure under a different provider is worse than none.
 */
export function planUsageFromReport(data: unknown): PlanUsage | undefined {
  if (!data || typeof data !== "object") return undefined;
  const report = (data as { report?: unknown }).report as PiUsageReport | undefined;
  if (!report || typeof report !== "object" || !report.semantics || typeof report.semantics !== "object") {
    return undefined;
  }
  return {
    subscription: report.semantics.kind === "consumer-subscription",
    rateLimits: rateLimitsFromUsageBuckets(report.buckets),
  };
}

/**
 * activeRateLimits picks which source feeds the usage widgets. Anthropic's
 * headers win while the model that produced them is still the current one:
 * they arrive on every response, so they are never older than pi-usage's
 * poll. Under any other provider they describe an account the session is no
 * longer using, and pi-usage, which follows the current model, decides.
 */
export function activeRateLimits(
  state: SessionState,
  provider: string | undefined,
): { rateLimits?: RateLimits; fromPlanUsage: boolean } {
  if (state.rateLimits && state.rateLimitsProvider === provider) {
    return { rateLimits: state.rateLimits, fromPlanUsage: false };
  }
  const plan = state.planUsage?.rateLimits;
  return plan ? { rateLimits: plan, fromPlanUsage: true } : { rateLimits: undefined, fromPlanUsage: false };
}

/** pi-lens's status key for its language-server line. */
export const LSP_STATUS_SLOT = "pi-lens-lsp";
/** pi-usage's status key. */
export const USAGE_STATUS_SLOT = "usage";

/**
 * absorbedStatusKeys names the other extensions' status lines this statusline
 * already draws as widgets, so the footer does not show them twice. pi-lens's
 * line always is, as the lsp widget. pi-usage's is only while its plan windows
 * are what the usage widgets are showing; an API-key balance has no widget,
 * so that line still earns its place under the footer.
 */
export function absorbedStatusKeys(state: SessionState, provider: string | undefined): Set<string> {
  const keys = new Set([LSP_STATUS_SLOT]);
  if (state.planUsage?.subscription && activeRateLimits(state, provider).fromPlanUsage) {
    keys.add(USAGE_STATUS_SLOT);
  }
  return keys;
}

/** The bits of a session entry this extension cares about. */
interface MaybeMessageEntry {
  type?: string;
  message?: { role?: string; usage?: PiUsage };
}

/**
 * sessionTotalsFromEntries replays a branch of session entries so a resumed
 * session starts with the right running cost and token breakdown rather than
 * zero. pi already priced every assistant message from its bundled models
 * catalogue, so cost is a sum, never a recomputation.
 */
export function sessionTotalsFromEntries(entries: unknown[] | undefined): {
  costUsd: number;
  lastUsage?: PiUsage;
} {
  let costUsd = 0;
  let lastUsage: PiUsage | undefined;
  if (!Array.isArray(entries)) return { costUsd };

  for (const raw of entries) {
    const entry = raw as MaybeMessageEntry;
    if (entry?.type !== "message") continue;
    const message = entry.message;
    if (message?.role !== "assistant") continue;
    const usage = message.usage;
    if (!usage) continue;
    costUsd += num(usage.cost?.total);
    lastUsage = usage;
  }
  return { costUsd, lastUsage };
}

// ---------------------------------------------------------------------------
// Process plumbing
// ---------------------------------------------------------------------------

export interface RunResult {
  stdout: string;
  stderr: string;
  code: number;
}

/**
 * runBinary pipes JSON into the statusline binary over stdin.
 *
 * pi's own `pi.exec()` (ExecOptions = { signal, timeout, cwd }) has no stdin
 * channel, so this goes through node's child_process directly. Extensions run
 * in-process with the user's full permissions, so that is available.
 */
export function runBinary(
  binary: string,
  args: string[],
  stdin: string,
  options: { cwd?: string; timeoutMs?: number } = {},
): Promise<RunResult> {
  return new Promise((resolve, reject) => {
    let child;
    try {
      child = spawn(binary, args, { cwd: options.cwd, stdio: ["pipe", "pipe", "pipe"] });
    } catch (err) {
      reject(err);
      return;
    }

    let stdout = "";
    let stderr = "";
    let settled = false;
    const finish = (fn: () => void) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      fn();
    };
    const timer = setTimeout(() => {
      finish(() => {
        child.kill("SIGKILL");
        reject(new Error("agent-statusline timed out"));
      });
    }, options.timeoutMs ?? 5000);
    timer.unref?.();

    child.stdout?.on("data", (d) => {
      stdout += String(d);
    });
    child.stderr?.on("data", (d) => {
      stderr += String(d);
    });
    child.on("error", (err) => finish(() => reject(err)));
    child.on("close", (code) => finish(() => resolve({ stdout, stderr, code: code ?? 0 })));
    // A closed stdin (binary exited early) must not raise EPIPE into pi.
    child.stdin?.on("error", () => {});
    child.stdin?.end(stdin);
  });
}

/** Best-effort read of pi's own version, for the `version` wire field. */
function readPiVersion(): string | undefined {
  try {
    const dir = process.env.PI_PACKAGE_DIR;
    if (!dir) return undefined;
    // Deliberately synchronous and guarded: it runs once, at load.
    const pkg = JSON.parse(readFileSync(`${dir}/package.json`, "utf8"));
    return typeof pkg?.version === "string" ? pkg.version : undefined;
  } catch {
    return undefined;
  }
}

// ---------------------------------------------------------------------------
// Extension entrypoint
// ---------------------------------------------------------------------------

// The data poll is a backstop for values no event announces: git porcelain,
// rate-limit countdown drift, the compaction counter. The repaint that keeps
// the spinner and the elapsed clocks alive is a separate, process-free timer
// inside the widget component.
const DATA_POLL_MS = 5000;

export default function (pi: any) {
  const state = newSessionState();
  state.version = readPiVersion();
  const binary = process.env.AGENT_STATUSLINE_BIN ?? "agent-statusline";
  let providerStartedAt = 0;
  let handle:
    | { setSnapshot(s: any): void; extensionStatus(key: string): string | undefined; dispose(): void }
    | undefined;
  let poll: ReturnType<typeof setInterval> | undefined;
  let inFlight = false;
  let pendingRefresh = false;
  // The bus hands its subscriber only the published data, so the context a
  // refresh needs has to be remembered from the last event that carried one.
  let lastCtx: PiExtensionContext | undefined;

  // Subscribed at load rather than at session_start, so a tally published
  // before the first frame is still on the next payload. An older pi with no
  // event bus simply leaves the widget hidden.
  let unsubscribeAutoMode: (() => void) | undefined;
  try {
    unsubscribeAutoMode = pi.events?.on?.(AUTO_MODE_STATUS_CHANNEL, (data: unknown) => {
      state.autoMode = autoModeText(data);
      if (lastCtx) void refresh(lastCtx);
    });
  } catch {
    // A bus that refuses subscribers costs one widget, never the statusline.
  }
  let unsubscribeUsage: (() => void) | undefined;
  try {
    unsubscribeUsage = pi.events?.on?.(PI_USAGE_REPORT_CHANNEL, (data: unknown) => {
      state.planUsage = planUsageFromReport(data);
      if (lastCtx) void refresh(lastCtx);
    });
  } catch {
    // Without pi-usage the usage widgets fall back to Anthropic's headers.
  }

  const syncSession = (ctx: PiExtensionContext) => {
    const sm = ctx?.sessionManager;
    if (!sm) return;
    try {
      state.sessionId = sm.getSessionId?.() ?? state.sessionId;
      state.sessionPath = sm.getSessionFile?.();
      state.sessionName = sm.getSessionName?.();
      state.projectDir = sm.getCwd?.() || undefined;
    } catch {
      // keep whatever we already had
    }
  };

  // session_start carries only { reason, previousSessionFile }; the id, file
  // and name all live on ctx.sessionManager.
  pi.on("session_start", (_event: any, ctx: PiExtensionContext) => {
    state.startedAt = Date.now();
    state.costUsd = 0;
    state.apiDurationMs = 0;
    state.lastUsage = undefined;
    syncSession(ctx);
    try {
      const totals = sessionTotalsFromEntries(ctx?.sessionManager?.getBranch?.() as unknown[]);
      state.costUsd = totals.costUsd;
      state.lastUsage = totals.lastUsage;
    } catch {
      // a fresh session has nothing to replay
    }
    install(ctx);
    void refresh(ctx);
  });

  pi.on("session_info_changed", (event: any, ctx: PiExtensionContext) => {
    state.sessionName = event?.name;
    void refresh(ctx);
  });

  // Assistant messages are the only place pi exposes a token breakdown, and
  // they arrive already priced from pi's bundled models catalogue.
  pi.on("message_end", (event: any, ctx: PiExtensionContext) => {
    const message = event?.message;
    if (message?.role === "assistant" && message.usage) {
      state.lastUsage = message.usage as PiUsage;
      state.costUsd += num(message.usage?.cost?.total);
      if (providerStartedAt > 0) {
        state.apiDurationMs += Math.max(0, Date.now() - providerStartedAt);
        providerStartedAt = 0;
      }
    }
    void refresh(ctx);
  });

  pi.on("turn_end", (_event: any, ctx: PiExtensionContext) => void refresh(ctx));
  pi.on("agent_settled", (_event: any, ctx: PiExtensionContext) => void refresh(ctx));

  pi.on("before_provider_request", () => {
    providerStartedAt = Date.now();
  });

  // Anthropic surfaces rate limits in response headers. Absent on Codex,
  // OpenRouter, and API-key auth, where only pi-usage's report can fill the
  // widgets. The provider is remembered with them so a later switch to another
  // provider stops them standing in for that provider's limits.
  pi.on("after_provider_response", (event: any, ctx: PiExtensionContext) => {
    const limits = rateLimitsFromHeaders(event?.headers);
    if (limits) {
      state.rateLimits = limits;
      state.rateLimitsProvider = ctx?.model?.provider;
    }
  });

  // Tool timing goes through the same sidecar Claude Code's hooks write. No
  // stdin is needed here, so pi's own exec is enough.
  const toolEvent = (event: any, phase: "start" | "end" | "fail") => {
    if (!state.sessionId) return;
    try {
      void Promise.resolve(
        pi.exec(binary, [
          "hook",
          "--mode", "pi",
          "--session", state.sessionId,
          "--tool", event?.toolName ?? "",
          "--call-id", event?.toolCallId ?? "",
          "--event", phase,
        ]),
      ).catch(() => {});
    } catch {
      // tool timing is a nicety, never a failure mode
    }
  };
  pi.on("tool_execution_start", (e: any) => toolEvent(e, "start"));
  pi.on("tool_execution_end", (e: any) => toolEvent(e, e?.isError ? "fail" : "end"));

  function install(ctx: PiExtensionContext) {
    lastCtx = ctx;
    handle?.dispose();
    // onDataStale fires on a branch change, which pi already watches and
    // debounces for us — cheaper and more responsive than polling git.
    handle = installStatusline(ctx as any, {
      onDataStale: () => void refresh(ctx),
      absorbedStatusKeys: () => absorbedStatusKeys(state, lastCtx?.model?.provider),
    });
    if (poll) clearInterval(poll);
    poll = setInterval(() => void refresh(ctx), DATA_POLL_MS);
    (poll as { unref?: () => void }).unref?.();
  }

  function teardown() {
    unsubscribeAutoMode?.();
    unsubscribeAutoMode = undefined;
    unsubscribeUsage?.();
    unsubscribeUsage = undefined;
    if (poll) clearInterval(poll);
    poll = undefined;
    handle?.dispose();
    handle = undefined;
  }

  pi.on("session_shutdown", () => teardown());

  async function refresh(ctx: PiExtensionContext) {
    lastCtx = ctx;
    // One binary at a time, but never a dropped update: a refresh that arrives
    // mid-flight is coalesced and re-run at the end, so the frame on screen
    // always reflects the last event rather than whichever one won a race.
    if (inFlight) {
      pendingRefresh = true;
      return;
    }
    inFlight = true;
    try {
      if (!state.sessionId) syncSession(ctx);
      // pi-lens publishes no event, only its status slot, and that is only
      // readable through the footer this extension took. A change shows up on
      // the next refresh, which the data poll bounds.
      state.lsp = handle?.extensionStatus(LSP_STATUS_SLOT);
      const payload = buildPayload(ctx, state);
      // No cwd override: the Go side reads the workspace out of the payload's
      // `cwd` field, and spawning into a directory that has since been removed
      // would fail the whole refresh.
      const result = await runBinary(binary, ["--mode", "pi", "--emit", "json"], JSON.stringify(payload));
      const snapshot = parseSnapshot(String(result.stdout ?? ""));
      // A malformed or newer-schema snapshot leaves the last good frame on
      // screen: a wrong statusline is worse than a stale one.
      if (snapshot) handle?.setSnapshot(snapshot);
    } catch {
      // A failed refresh leaves the previous frame in place.
    } finally {
      inFlight = false;
      if (pendingRefresh) {
        pendingRefresh = false;
        void refresh(ctx);
      }
    }
  }
}
