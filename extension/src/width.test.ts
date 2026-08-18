import { describe, expect, it } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import {
  truncateToWidth as piTruncateToWidth,
  visibleWidth as piVisibleWidth,
} from "@earendil-works/pi-tui";

import { padTo, truncateEnd, truncateMiddle, visibleWidth } from "./width";

const corpus: string[] = JSON.parse(
  readFileSync(join(import.meta.dir, "..", "testdata", "width-corpus.json"), "utf8"),
);

describe("visibleWidth", () => {
  // pi-tui's visibleWidth is what pi's own differential renderer measures with.
  // If ours disagrees by one cell, our lines overflow or under-fill and the
  // frame corrupts. So pi is the authority and we are pinned to it.
  it("agrees with pi-tui on every corpus entry", () => {
    for (const s of corpus) {
      expect([s, visibleWidth(s)]).toEqual([s, piVisibleWidth(s)]);
    }
  });

  it("agrees with pi-tui on fuzzed unicode", () => {
    let seed = 20260818;
    const rand = () => (seed = (seed * 1103515245 + 12345) % 2147483648) / 2147483648;
    for (let i = 0; i < 2000; i++) {
      let s = "";
      for (let j = 0; j < 1 + Math.floor(rand() * 12); j++) {
        const cp = Math.floor(rand() * 0x2ffff);
        if (cp >= 0xd800 && cp <= 0xdfff) continue;
        s += String.fromCodePoint(cp);
      }
      expect([s, visibleWidth(s)]).toEqual([s, piVisibleWidth(s)]);
    }
  });

  it("ignores OSC 8 hyperlink payloads", () => {
    const link = "\x1b]8;;https://example.test/pr/12\x1b\\#12\x1b]8;;\x1b\\";
    expect(visibleWidth(link)).toBe(3);
  });
});

describe("truncateEnd", () => {
  it("never exceeds the budget, measured by pi-tui", () => {
    for (const s of corpus) {
      for (const max of [0, 1, 3, 7, 20, 200]) {
        expect(piVisibleWidth(truncateEnd(s, max))).toBeLessThanOrEqual(max);
      }
    }
  });

  it("matches pi-tui's own truncateToWidth for plain text", () => {
    for (const s of corpus.filter((c) => !c.includes("\x1b") && !c.includes("\t"))) {
      expect(truncateEnd(s, 10)).toBe(piTruncateToWidth(s, 10));
    }
  });

  it("matches pi-tui's own truncateToWidth for escaped text too", () => {
    // The escaped half of the corpus is where a hand-rolled truncator goes
    // wrong: an OSC 8 cut without its close leaves the rest of the line linked.
    for (const s of corpus) {
      for (const max of [1, 4, 12, 40]) {
        expect([s, max, truncateEnd(s, max)]).toEqual([s, max, piTruncateToWidth(s, max)]);
      }
    }
  });

  it("leaves a string that already fits untouched", () => {
    expect(truncateEnd("main", 10)).toBe("main");
  });

  it("produces lines that are fixed points of pi's truncator", () => {
    for (const s of corpus) {
      const cut = truncateEnd(s, 12);
      expect([s, piTruncateToWidth(cut, 12)]).toEqual([s, cut]);
    }
  });
});

describe("truncateMiddle", () => {
  it("keeps the head and the tail", () => {
    const got = truncateMiddle("/home/joe/Development/agent-statusline/internal/render", 20);
    expect(visibleWidth(got)).toBeLessThanOrEqual(20);
    expect(got.startsWith("/home")).toBe(true);
    expect(got.endsWith("render")).toBe(true);
  });

  it("degrades to an end truncation when the budget is tiny", () => {
    expect(visibleWidth(truncateMiddle("abcdefgh", 2))).toBeLessThanOrEqual(2);
  });

  it("never exceeds the budget on any corpus entry, measured by pi-tui", () => {
    for (const s of corpus) {
      for (const max of [1, 5, 13, 40]) {
        expect([s, max, piVisibleWidth(truncateMiddle(s, max)) <= max]).toEqual([s, max, true]);
      }
    }
  });
});

describe("padTo", () => {
  it("pads to exactly the requested width", () => {
    expect(piVisibleWidth(padTo("main", 10))).toBe(10);
  });

  it("pads wide-glyph strings to exactly the requested width", () => {
    expect(piVisibleWidth(padTo("日本語", 10))).toBe(10);
  });

  it("never shrinks a string that is already wider", () => {
    expect(padTo("main", 2)).toBe("main");
  });
});
