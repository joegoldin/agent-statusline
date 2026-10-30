import type { Span } from "./snapshot";

const CLOCK = "\uf017";
const BOLT = "\uf0e7";
const HISTORY = 12;

function seconds(ms: number): string {
  const s = Math.max(0, ms) / 1000;
  return s < 10 ? `${s.toFixed(1)}s` : s < 1000 ? `${Math.floor(s)}s` : s < 60000 ? `${Math.floor(s / 60)}m` : "999m+";
}

function rate(tokens: number, ms: number): string {
  if (ms <= 0 || tokens <= 0) return "—";
  const n = tokens * 1000 / ms;
  return n < 1000 ? String(Math.round(n)) : n < 10000 ? `${(n / 1000).toFixed(1)}k` : "9k+";
}

/** Per-request timing, not tool time or total agent-turn time. */
export class ResponsePerformance {
  private history = new Map<string, number[]>();
  private request?: { key: string; start: number; first?: number; chars: number; end?: number; tokens?: number; failed?: boolean };

  reset(): void { this.request = undefined; this.history.clear(); }

  start(key: string, now: number): void {
    this.request = { key, start: now, chars: 0 };
  }

  delta(text: string, now: number): void {
    const r = this.request;
    if (!r || r.end !== undefined || !text) return;
    r.first ??= now;
    r.chars += [...text].length;
  }

  finish(tokens: number | undefined, failed: boolean, now: number): void {
    const r = this.request;
    if (!r || r.end !== undefined) return;
    r.end = now;
    r.failed = failed;
    r.tokens = typeof tokens === "number" && Number.isFinite(tokens) && tokens > 0 ? tokens : undefined;
    if (!failed && r.first !== undefined) {
      const samples = this.history.get(r.key) ?? [];
      samples.push(Math.max(0, r.first - r.start));
      this.history.delete(r.key);
      this.history.set(r.key, samples.slice(-HISTORY));
      if (this.history.size > 32) this.history.delete(this.history.keys().next().value!);
    }
  }

  render(now: number): Span[] {
    const r = this.request;
    if (!r) return [];
    const elapsed = Math.max(0, (r.end ?? now) - r.start);
    let text: string;
    let intent: Span["intent"] = "dim";
    if (r.failed) {
      text = `\uf071 ${seconds(elapsed)} stopped`;
      intent = "warn";
    } else if (r.first === undefined) {
      const samples = this.history.get(r.key) ?? [];
      const estimate = samples.reduce((a, b) => a + b, 0) / samples.length;
      if (r.end === undefined && estimate > 0 && elapsed < estimate) {
        const filled = Math.min(3, Math.floor(elapsed / estimate * 4));
        text = `${CLOCK} ${"▰".repeat(filled)}${"▱".repeat(4 - filled)} ${seconds(elapsed)}`;
      } else {
        text = `${CLOCK} ${seconds(elapsed)}${r.end === undefined ? " …" : " —"}`;
      }
    } else if (r.end === undefined) {
      // Providers generally publish usage only at the end. Character/4 is a
      // deliberately labelled estimate, never a count of stream chunks.
      text = `${BOLT}~${rate(r.chars / 4, now - r.first)}t/s ${seconds(elapsed)}`;
      intent = "accent";
    } else {
      text = `${CLOCK}${seconds(r.first - r.start)} ${BOLT}${rate(r.tokens ?? 0, r.end - r.first)}t/s`;
    }
    return [{ kind: "text", intent, text }];
  }
}

/** Event wiring is separate so timing tests need neither pi nor a subprocess. */
export function registerPerformance(pi: { on(name: string, handler: (event: any, ctx: any) => void): unknown }, repaint: () => void, now = () => performance.now()): ResponsePerformance {
  const timing = new ResponsePerformance();
  pi.on("session_start", () => { timing.reset(); repaint(); });
  pi.on("before_provider_request", (_event, ctx) => {
    timing.start(`${ctx?.model?.provider ?? ""}/${ctx?.model?.id ?? ""}`, now());
    repaint();
  });
  pi.on("message_update", event => {
    const delta = event?.assistantMessageEvent;
    if (["text_delta", "thinking_delta", "toolcall_delta"].includes(delta?.type) && typeof delta.delta === "string") {
      timing.delta(delta.delta, now());
      repaint();
    }
  });
  pi.on("message_end", event => {
    const message = event?.message;
    if (message?.role !== "assistant") return;
    timing.finish(message.usage?.output, ["error", "aborted"].includes(message.stopReason), now());
    repaint();
  });
  pi.on("agent_end", () => { timing.finish(undefined, true, now()); repaint(); });
  pi.on("session_shutdown", () => timing.reset());
  return timing;
}
