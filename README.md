# agent-statusline

A statusline for terminal coding agents. Renders the same line for
[Claude Code](https://claude.com/claude-code) and [pi](https://pi.dev), from one
config schema.

## Modes and encodings

The binary reads a JSON payload on stdin and writes a statusline to stdout.

`--mode` selects the **input** decoder:

- `--mode claude` — Claude Code's statusline stdin JSON
- `--mode pi` — the payload emitted by this repo's pi extension
- omitted, or `--mode auto` — detected from the payload's `harness` field

`--emit` selects the **output** encoding:

- `--emit ansi` (default) — a rendered, coloured statusline. What Claude Code
  consumes, and the quickest way to eyeball what the binary computed.
- `--emit json` — a structured snapshot: per-widget spans carrying text and a
  semantic colour *intent*, bars carrying a fill fraction, activity items
  carrying absolute timestamps, plus the effective config. What the pi
  extension consumes, so that pi can do layout and colour in its own theme, at
  its own width, on its own clock.

`--emit ansi` is deliberately retained for `--mode pi`. It is the only rendering
available in pi's non-TUI modes, where `setWidget` and `setFooter` are no-ops,
and it is how you tell a Go bug from a TypeScript one: run the binary by hand
with a captured payload and see whether the numbers are right before blaming
the renderer. `internal/e2e/testdata/pi-*.golden` keep it under test.

## Consumers

- `claude-nix` sets `settings.statusLine.command` to this binary
- `pi-nix` loads `extension/statusline.ts`, which shells out to it

Both mount the shared option schema from this flake's `lib`, so a widget added
here appears in both agents:

```nix
lib.${system}.statuslineOptions   # attrset of mkOptions, mount under any namespace
lib.${system}.renderConfig cfg    # evaluated options -> config JSON derivation
```

## Development

```sh
nix build .#agent-statusline
nix flake check
cd extension && bun test
```

Golden tests live in `internal/e2e/testdata/` and `extension/testdata/`. The
Claude-mode goldens are a regression gate and must never be regenerated
casually — if that output changes, that is the bug.

`extension/testdata/snapshot-full.json` is the binary's real `--emit json`
output, not a hand-written fixture, so the TypeScript suite breaks the moment
the Go schema moves. Regenerate it with a clean cache, a fixed clock and a
seeded tool-timing sidecar, or the activity rows come out empty:

```sh
H=$(mktemp -d) && mkdir -p "$H/.cache/agent-statusline/tool-timing"
# write $H/.cache/agent-statusline/tool-timing/pi-golden-1.json with tools whose
# timestamps are offsets from 1748260800, then:
CLAUDE_STATUSLINE_NOW=1748260800 CLAUDE_STATUSLINE_CONFIG=/dev/null \
  HOME=$H XDG_CACHE_HOME=$H/.cache \
  PI_CODING_AGENT_DIR=$PWD/internal/e2e/testdata/pi-agent \
  ./agent-statusline --emit json < internal/e2e/testdata/pi-full.json \
  > extension/testdata/snapshot-full.json
cd extension && UPDATE_GOLDEN=1 bun test src/rows.test.ts
```

`PI_CODING_AGENT_DIR` matters for the same reason: the cache row reads pi's
state directory, and pointing it at the checked-in fixture is what keeps that
row out of whatever the machine running the tests has cached.

`XDG_CACHE_HOME` matters as much as `HOME`: `os.UserCacheDir` prefers it, so
setting only `HOME` reads your real cache and the output stops being
reproducible.
