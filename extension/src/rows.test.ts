import { describe, expect, it } from "bun:test";
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import { renderRows } from "./layout";
import { parseSnapshot, type Snapshot } from "./snapshot";
import { recordingTheme } from "./testing";

const snap = parseSnapshot(
  readFileSync(join(import.meta.dir, "..", "testdata", "snapshot-full.json"), "utf8"),
) as Snapshot;

const NOW = 1748260800000;
const UPDATE = process.env.UPDATE_GOLDEN === "1";

// Visualise escapes the way internal/e2e/golden_test.go does, so a diff in
// either language reads the same way.
const visualise = (s: string) => s.replaceAll("\x1b", "<ESC>");

describe("golden rows", () => {
  for (const width of [40, 120]) {
    it(`matches testdata/rows-${width}.golden`, () => {
      const { theme } = recordingTheme();
      const got = renderRows(snap, width, theme, NOW).map(visualise).join("\n") + "\n";
      const path = join(import.meta.dir, "..", "testdata", `rows-${width}.golden`);
      if (UPDATE) {
        writeFileSync(path, got);
        return;
      }
      expect(got).toBe(readFileSync(path, "utf8"));
    });
  }
});
