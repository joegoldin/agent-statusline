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
  model?: { id: string; display_name: string };
  thinking_level?: string;
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
  rateLimits?: RateLimits;
  version?: string;
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
    rate_limits: state.rateLimits,
    version: state.version,
  };

  // pi-ai Model exposes `name`, not `displayName`.
  if (ctx?.model?.id) {
    payload.model = { id: ctx.model.id, display_name: ctx.model.name ?? ctx.model.id };
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
 * auth, in which case the widgets stay hidden.
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

const STATUS_KEY = "agent-statusline";

export default function (pi: any) {
  const state = newSessionState();
  state.version = readPiVersion();
  const binary = process.env.AGENT_STATUSLINE_BIN ?? "agent-statusline";
  let providerStartedAt = 0;

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
  // OpenRouter, and API-key auth, in which case the widgets stay hidden.
  pi.on("after_provider_response", (event: any) => {
    const limits = rateLimitsFromHeaders(event?.headers);
    if (limits) state.rateLimits = limits;
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

  async function refresh(ctx: PiExtensionContext) {
    try {
      if (!state.sessionId) syncSession(ctx);
      const payload = buildPayload(ctx, state);
      // No cwd override: the Go side reads the workspace out of the payload's
      // `cwd` field, and spawning into a directory that has since been removed
      // would fail the whole refresh.
      const result = await runBinary(binary, ["--mode", "pi"], JSON.stringify(payload));
      const line = String(result.stdout ?? "").replace(/\n+$/, "");
      if (line) ctx?.ui?.setStatus?.(STATUS_KEY, line);
    } catch {
      // A failed refresh leaves the previous line in place.
    }
  }
}
