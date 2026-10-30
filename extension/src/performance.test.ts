import { expect, test } from "bun:test";
import { ResponsePerformance, registerPerformance } from "./performance";
import { visibleWidth } from "./width";

const text = (p: ResponsePerformance, now: number) => p.render(now).map(s => s.text).join("");

test("wait, estimated streaming, final TTFT and measured speed", () => {
  const p = new ResponsePerformance();
  expect(text(p, 0)).toBe("");
  p.start("openai/a", 0);
  expect(text(p, 500)).toBe("\uf017 0.5s …");
  p.delta("", 1000);
  p.delta("x".repeat(400), 2000);
  expect(text(p, 3000)).toBe("\uf0e7~100t/s 3.0s");
  p.finish(200, false, 4000);
  expect(text(p, 99999)).toBe("\uf0172.0s \uf0e7100t/s");
  p.start("openai/a", 5000);
  expect(text(p, 6000)).toBe("\uf017 ▰▰▱▱ 1.0s");
  expect(text(p, 8000)).toBe("\uf017 3.0s …");
  p.start("openai/b", 8000);
  expect(text(p, 9000)).toBe("\uf017 1.0s …");
});

test("failures and absent usage never invent a rate or train latency", () => {
  const p = new ResponsePerformance();
  p.start("a", 0);
  p.delta("reasoning", 1000);
  p.finish(undefined, true, 2000);
  expect(text(p, 2000)).toContain("stopped");
  p.start("a", 3000);
  expect(text(p, 3500)).not.toContain("▱");
  p.delta("answer", 4000);
  p.finish(undefined, false, 5000);
  expect(text(p, 5000)).toContain("—t/s");
  p.reset();
  expect(text(p, 6000)).toBe("");
});

test("all phases fit 16 terminal cells, even for long waits and high rates", () => {
  for (const elapsed of [0, 100, 1000, 9999, 99999, 999999, 999999999]) {
    const p = new ResponsePerformance();
    p.start("a", 0);
    expect(visibleWidth(text(p, elapsed))).toBeLessThanOrEqual(16);
    p.delta("x".repeat(100000), elapsed);
    expect(visibleWidth(text(p, elapsed + 1))).toBeLessThanOrEqual(16);
    p.finish(100000, false, elapsed + 1);
    expect(visibleWidth(text(p, elapsed + 1))).toBeLessThanOrEqual(16);
  }
});

test("rolling estimate keeps only recent successful requests", () => {
  const p = new ResponsePerformance();
  for (let i = 0; i < 13; i++) {
    p.start("a", 0);
    p.delta("x", i === 0 ? 100000 : 2000);
    p.finish(10, false, 101000);
  }
  p.start("a", 0);
  expect(text(p, 1000)).toContain("▰▰▱▱");
});

test("wiring includes hidden thinking and tool arguments, excludes tool execution and handles abort", () => {
  const handlers = new Map<string, (e: any, c?: any) => void>();
  let now = 0;
  let paints = 0;
  const p = registerPerformance({ on: (name, fn) => handlers.set(name, fn) }, () => paints++, () => now);
  const emit = (name: string, e: any = {}) => handlers.get(name)?.(e, { model: { provider: "p", id: "m" } });
  emit("session_start");
  emit("before_provider_request");
  now = 2000;
  emit("message_update", { assistantMessageEvent: { type: "start" } });
  emit("message_update", { assistantMessageEvent: { type: "thinking_delta", delta: "thought" } });
  now = 3000;
  emit("message_update", { assistantMessageEvent: { type: "toolcall_delta", delta: "{}" } });
  emit("message_end", { message: { role: "assistant", usage: { output: 80 }, stopReason: "toolUse" } });
  now = 100000;
  emit("message_end", { message: { role: "toolResult" } });
  emit("agent_end");
  expect(text(p, now)).toBe("\uf0172.0s \uf0e780t/s");
  emit("before_provider_request");
  now += 1000;
  emit("agent_end");
  expect(text(p, now)).toContain("stopped");
  expect(paints).toBeGreaterThan(0);
});
