import { chmodSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe, expect, it, vi } from "vitest";

import {
  buildPayload,
  newSessionState,
  rateLimitsFromHeaders,
  runBinary,
  sessionTotalsFromEntries,
  type PiExtensionContext,
  type SessionState,
} from "./statusline";

// The fixtures below use pi's REAL shapes, verified against
// @earendil-works/pi-coding-agent/core/extensions/types.d.ts and
// @earendil-works/pi-ai/dist/types.d.ts in the built package:
//   ContextUsage = { tokens, contextWindow, percent }   (no token breakdown)
//   Model        = { id, name, contextWindow, cost, ... } (no displayName)
//   Usage        = { input, output, cacheRead, cacheWrite, cost: { total } }

const ctx: PiExtensionContext = {
  cwd: "/home/joe/p",
  model: { id: "gpt-5.6-sol", name: "Sol", contextWindow: 400000 },
  thinkingLevel: "xhigh",
  getContextUsage: () => ({ tokens: 214000, contextWindow: 400000, percent: 53.5 }),
};

function state(over: Partial<SessionState> = {}): SessionState {
  return { ...newSessionState(0), sessionId: "s1", ...over };
}

const lastUsage = {
  input: 120000,
  output: 8000,
  cacheRead: 90000,
  cacheWrite: 4000,
  totalTokens: 222000,
  cost: { input: 0.3, output: 0.1, cacheRead: 0.01, cacheWrite: 0.01, total: 0.42 },
};

describe("buildPayload", () => {
  it("stamps the harness discriminator so autodetect works", () => {
    expect(buildPayload(ctx, state(), 0).harness).toBe("pi");
  });

  it("maps model and thinking level onto the wire format", () => {
    const p = buildPayload(ctx, state(), 0);
    // pi's Model field is `name`; the wire field is `display_name`.
    expect(p.model).toEqual({ id: "gpt-5.6-sol", display_name: "Sol" });
    expect(p.thinking_level).toBe("xhigh");
  });

  it("falls back to the model id when pi reports no display name", () => {
    const p = buildPayload({ ...ctx, model: { id: "bare-model" } }, state(), 0);
    expect(p.model).toEqual({ id: "bare-model", display_name: "bare-model" });
  });

  it("passes the assistant token breakdown through without deriving percentages", () => {
    const p = buildPayload(ctx, state({ lastUsage }), 0);
    expect(p.context).toEqual({
      window_size: 400000,
      input_tokens: 120000,
      output_tokens: 8000,
      cache_read_tokens: 90000,
      // pi calls the cache-creation bucket `cacheWrite`.
      cache_creation_tokens: 4000,
    });
    expect(p).not.toHaveProperty("used_percentage");
  });

  it("uses pi's context estimate before any assistant message has landed", () => {
    const p = buildPayload(ctx, state(), 0);
    expect(p.context).toEqual({
      window_size: 400000,
      input_tokens: 214000,
      output_tokens: 0,
      cache_read_tokens: 0,
      cache_creation_tokens: 0,
    });
  });

  it("takes the window size from the model when getContextUsage is unavailable", () => {
    const p = buildPayload({ ...ctx, getContextUsage: undefined }, state({ lastUsage }), 0);
    expect(p.context?.window_size).toBe(400000);
  });

  it("computes elapsed duration from the session start", () => {
    const p = buildPayload(ctx, state({ startedAt: 1000 }), 4000);
    expect(p.duration_ms).toBe(3000);
  });

  it("carries the session identity that lives on ctx.sessionManager", () => {
    const p = buildPayload(
      ctx,
      state({
        sessionId: "pi-abc-123",
        sessionName: "fork-the-flake",
        sessionPath: "/home/joe/.pi/agent/sessions/pi-abc-123.jsonl",
        projectDir: "/home/joe/p",
      }),
      0,
    );
    expect(p.session_id).toBe("pi-abc-123");
    expect(p.session_name).toBe("fork-the-flake");
    expect(p.session_path).toBe("/home/joe/.pi/agent/sessions/pi-abc-123.jsonl");
    expect(p.project_dir).toBe("/home/joe/p");
  });

  it("omits rate_limits when no Anthropic headers were seen", () => {
    expect(buildPayload(ctx, state(), 0).rate_limits).toBeUndefined();
  });

  it("omits cost and api duration until they are non-zero", () => {
    const p = buildPayload(ctx, state(), 0);
    expect(p.cost_usd).toBeUndefined();
    expect(p.api_duration_ms).toBeUndefined();

    const q = buildPayload(ctx, state({ costUsd: 0.42, apiDurationMs: 1200 }), 0);
    expect(q.cost_usd).toBe(0.42);
    expect(q.api_duration_ms).toBe(1200);
  });

  it("tolerates a context usage call that throws", () => {
    const broken: PiExtensionContext = {
      cwd: "/home/joe/p",
      getContextUsage: () => {
        throw new Error("nope");
      },
    };
    const p = buildPayload(broken, state(), 0);
    expect(p.context).toBeUndefined();
    expect(p.harness).toBe("pi");
  });

  it("survives an empty context object", () => {
    const p = buildPayload({}, state(), 0);
    expect(p.harness).toBe("pi");
    expect(p.cwd).toBe("");
    expect(p.model).toBeUndefined();
  });

  it("serialises to exactly the JSON tags the Go decoder expects", () => {
    const p = buildPayload(ctx, state({ lastUsage, costUsd: 0.42, startedAt: 0 }), 330000);
    const wire = JSON.parse(JSON.stringify(p));
    expect(Object.keys(wire).sort()).toEqual(
      ["context", "cost_usd", "cwd", "duration_ms", "harness", "model", "session_id", "thinking_level"].sort(),
    );
    expect(Object.keys(wire.context).sort()).toEqual(
      [
        "cache_creation_tokens",
        "cache_read_tokens",
        "input_tokens",
        "output_tokens",
        "window_size",
      ].sort(),
    );
    expect(wire.duration_ms).toBe(330000);
  });
});

describe("rateLimitsFromHeaders", () => {
  it("returns undefined when the provider sent no unified headers", () => {
    expect(rateLimitsFromHeaders(undefined)).toBeUndefined();
    expect(rateLimitsFromHeaders({ "content-type": "application/json" })).toBeUndefined();
  });

  it("reads the 5h and 7d unified windows case-insensitively", () => {
    const limits = rateLimitsFromHeaders({
      "Anthropic-RateLimit-Unified-5h-Used-Percentage": "12.5",
      "anthropic-ratelimit-unified-5h-reset": "1748260800",
      "anthropic-ratelimit-unified-7d-used-percentage": "80",
      "anthropic-ratelimit-unified-7d-reset": "1748860800",
    });
    expect(limits).toEqual({
      five_hour: { used_percentage: 12.5, resets_at: 1748260800 },
      seven_day: { used_percentage: 80, resets_at: 1748860800 },
    });
  });

  it("defaults a missing reset to zero rather than NaN", () => {
    const limits = rateLimitsFromHeaders({
      "anthropic-ratelimit-unified-5h-used-percentage": "3",
    });
    expect(limits?.five_hour).toEqual({ used_percentage: 3, resets_at: 0 });
  });
});

describe("sessionTotalsFromEntries", () => {
  const entry = (usage: unknown) => ({
    type: "message",
    id: "e",
    parentId: null,
    timestamp: "",
    message: { role: "assistant", usage },
  });

  it("sums the cost pi already priced onto each assistant message", () => {
    const totals = sessionTotalsFromEntries([
      entry({ input: 10, output: 1, cacheRead: 0, cacheWrite: 0, cost: { total: 0.1 } }),
      { type: "message", message: { role: "user", content: "hi" } },
      entry(lastUsage),
    ]);
    expect(totals.costUsd).toBeCloseTo(0.52, 10);
    expect(totals.lastUsage).toEqual(lastUsage);
  });

  it("returns zeroes for a fresh or unreadable session", () => {
    expect(sessionTotalsFromEntries(undefined)).toEqual({ costUsd: 0, lastUsage: undefined });
    expect(sessionTotalsFromEntries([])).toEqual({ costUsd: 0, lastUsage: undefined });
  });

  it("ignores assistant messages with no usage attached", () => {
    const totals = sessionTotalsFromEntries([
      { type: "message", message: { role: "assistant" } },
      { type: "compaction", summary: "x" },
    ]);
    expect(totals).toEqual({ costUsd: 0, lastUsage: undefined });
  });
});

describe("runBinary", () => {
  // pi.exec has no stdin channel (ExecOptions = { signal, timeout, cwd }), so
  // the payload goes over node's child_process instead.
  it("pipes the payload in over stdin and returns stdout", async () => {
    const result = await runBinary("cat", [], JSON.stringify({ harness: "pi" }));
    expect(result.code).toBe(0);
    expect(JSON.parse(result.stdout)).toEqual({ harness: "pi" });
  });

  it("rejects rather than throwing into pi when the binary is missing", async () => {
    await expect(
      runBinary("agent-statusline-does-not-exist", [], "{}"),
    ).rejects.toBeInstanceOf(Error);
  });

  it("does not raise EPIPE when the binary exits before reading stdin", async () => {
    const result = await runBinary("true", [], "x".repeat(1_000_000));
    expect(result.code).toBe(0);
  });
});

describe("extension entrypoint", () => {
  // Drives the real default export against a fake ExtensionAPI shaped like
  // pi's, to prove the event wiring and the exec plumbing hold together.
  function fakePi() {
    const handlers = new Map<string, Function>();
    const execCalls: Array<{ command: string; args: string[] }> = [];
    return {
      handlers,
      execCalls,
      on(event: string, handler: Function) {
        handlers.set(event, handler);
      },
      exec(command: string, args: string[]) {
        execCalls.push({ command, args });
        return Promise.resolve({ stdout: "", stderr: "", code: 0, killed: false });
      },
      emit(event: string, payload: unknown, c: unknown) {
        return handlers.get(event)?.(payload, c);
      },
    };
  }

  function fakeCtx() {
    const statuses: Array<[string, string | undefined]> = [];
    return {
      statuses,
      ctx: {
        cwd: "/home/joe/p",
        model: { id: "gpt-5.6-sol", name: "Sol", contextWindow: 400000 },
        thinkingLevel: "high",
        getContextUsage: () => ({ tokens: 100, contextWindow: 400000, percent: 0.025 }),
        sessionManager: {
          getCwd: () => "/home/joe/p",
          getSessionId: () => "pi-abc-123",
          getSessionFile: () => "/home/joe/.pi/agent/sessions/pi-abc-123.jsonl",
          getSessionName: () => "fork-the-flake",
          getBranch: () => [],
        },
        ui: {
          setStatus: (key: string, text: string | undefined) => {
            statuses.push([key, text]);
          },
        },
      } as PiExtensionContext,
    };
  }

  it("subscribes to the events pi actually publishes", async () => {
    const { default: register } = await import("./statusline");
    const pi = fakePi();
    register(pi);
    for (const event of [
      "session_start",
      "session_info_changed",
      "message_end",
      "turn_end",
      "agent_settled",
      "before_provider_request",
      "after_provider_response",
      "tool_execution_start",
      "tool_execution_end",
    ]) {
      expect(pi.handlers.has(event), event).toBe(true);
    }
  });

  // A stand-in for the Go binary: ignores argv, echoes stdin. Lets the tests
  // assert on the exact payload the extension puts on the wire.
  function echoBinary(): string {
    const dir = mkdtempSync(join(tmpdir(), "agent-statusline-"));
    const path = join(dir, "fake-statusline");
    writeFileSync(path, "#!/bin/sh\nexec cat\n");
    chmodSync(path, 0o755);
    return path;
  }

  it("renders whatever the binary prints into ctx.ui.setStatus", async () => {
    process.env.AGENT_STATUSLINE_BIN = echoBinary();
    try {
      const { default: register } = await import("./statusline");
      const pi = fakePi();
      register(pi);
      const { ctx, statuses } = fakeCtx();
      // Handlers fire the refresh without awaiting it: a statusline must never
      // block the session, so the assertion polls instead.
      await pi.emit("session_start", { type: "session_start", reason: "startup" }, ctx);
      await vi.waitFor(() => expect(statuses.length).toBe(1));
      const [key, text] = statuses[0];
      expect(key).toBe("agent-statusline");
      const wire = JSON.parse(String(text));
      expect(wire.harness).toBe("pi");
      expect(wire.session_id).toBe("pi-abc-123");
      expect(wire.session_path).toBe("/home/joe/.pi/agent/sessions/pi-abc-123.jsonl");
      expect(wire.session_name).toBe("fork-the-flake");
      expect(wire.model).toEqual({ id: "gpt-5.6-sol", display_name: "Sol" });
      expect(wire.thinking_level).toBe("high");
    } finally {
      delete process.env.AGENT_STATUSLINE_BIN;
    }
  });

  it("forwards tool timing to the hook subcommand with pi's field names", async () => {
    const { default: register } = await import("./statusline");
    const pi = fakePi();
    register(pi);
    const { ctx } = fakeCtx();
    await pi.emit("session_start", { type: "session_start", reason: "startup" }, ctx);
    await pi.emit(
      "tool_execution_start",
      { type: "tool_execution_start", toolCallId: "call-1", toolName: "bash", args: {} },
      ctx,
    );
    await pi.emit(
      "tool_execution_end",
      { type: "tool_execution_end", toolCallId: "call-1", toolName: "bash", result: {}, isError: false },
      ctx,
    );
    expect(pi.execCalls.map((c) => c.args)).toEqual([
      ["hook", "--mode", "pi", "--session", "pi-abc-123", "--tool", "bash", "--call-id", "call-1", "--event", "start"],
      ["hook", "--mode", "pi", "--session", "pi-abc-123", "--tool", "bash", "--call-id", "call-1", "--event", "end"],
    ]);
  });

  it("marks a failed tool execution as fail, not end", async () => {
    const { default: register } = await import("./statusline");
    const pi = fakePi();
    register(pi);
    const { ctx } = fakeCtx();
    await pi.emit("session_start", { type: "session_start", reason: "startup" }, ctx);
    await pi.emit(
      "tool_execution_end",
      { type: "tool_execution_end", toolCallId: "c", toolName: "bash", result: {}, isError: true },
      ctx,
    );
    expect(pi.execCalls[0].args.slice(-2)).toEqual(["--event", "fail"]);
  });

  it("accumulates cost and rate limits onto the next payload", async () => {
    process.env.AGENT_STATUSLINE_BIN = echoBinary();
    try {
      const { default: register } = await import("./statusline");
      const pi = fakePi();
      register(pi);
      const { ctx, statuses } = fakeCtx();
      await pi.emit("session_start", { type: "session_start", reason: "startup" }, ctx);
      await pi.emit("before_provider_request", { type: "before_provider_request", payload: {} }, ctx);
      await new Promise((r) => setTimeout(r, 5));
      await pi.emit(
        "message_end",
        { type: "message_end", message: { role: "assistant", usage: lastUsage } },
        ctx,
      );
      await pi.emit(
        "after_provider_response",
        {
          type: "after_provider_response",
          status: 200,
          headers: { "anthropic-ratelimit-unified-5h-used-percentage": "12.5" },
        },
        ctx,
      );
      await pi.emit("agent_settled", { type: "agent_settled" }, ctx);

      await vi.waitFor(() => {
        const last = statuses.at(-1);
        expect(last).toBeDefined();
        const wire = JSON.parse(String(last?.[1]));
        expect(wire.cost_usd).toBeCloseTo(0.42, 10);
        expect(wire.rate_limits).toEqual({ five_hour: { used_percentage: 12.5, resets_at: 0 } });
        // The assistant breakdown replaces pi's single-number estimate.
        expect(wire.context).toEqual({
          window_size: 400000,
          input_tokens: 120000,
          output_tokens: 8000,
          cache_read_tokens: 90000,
          cache_creation_tokens: 4000,
        });
        // Measured between before_provider_request and the assistant message_end.
        expect(wire.api_duration_ms).toBeGreaterThan(0);
      });
    } finally {
      delete process.env.AGENT_STATUSLINE_BIN;
    }
  });
});
