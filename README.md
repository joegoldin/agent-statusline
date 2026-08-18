# agent-statusline

A statusline for terminal coding agents. Renders the same line for
[Claude Code](https://claude.com/claude-code) and [pi](https://pi.dev), from one
config schema.

## Modes

The binary reads a JSON payload on stdin and renders a statusline to stdout.

- `--mode claude` — Claude Code's statusline stdin JSON
- `--mode pi` — the payload emitted by this repo's pi extension
- omitted, or `--mode auto` — detected from the payload's `harness` field

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
```

Golden tests live in `internal/e2e/testdata/`. The Claude-mode goldens are a
regression gate and must never be regenerated casually — if output changes,
that is the bug.
