{ pkgs, lib }:
# Renders evaluated statusline options into the JSON the Go binary reads
# (`internal/config`.Config, loaded from ~/.claude/statusline-config.json or
# $AGENT_STATUSLINE_CONFIG).
#
# Only the keys config.Config declares are emitted; `enable`, `package`,
# `hideVimModeIndicator` and `toolTiming` are harness-level concerns and are
# deliberately excluded. The Go decoder tolerates unknown keys, but emitting
# extras would make the drift check in tests/options-test.nix meaningless.
#
# The result is a `writeText` derivation, so its JSON is readable purely as
# `(renderConfig cfg).text` — no import-from-derivation needed to inspect it.
cfg:
pkgs.writeText "agent-statusline-config.json" (
  builtins.toJSON {
    inherit (cfg)
      padding
      refreshInterval
      activityRows
      hideWhenIdle
      gitCacheTtlSeconds
      transcriptWindowSeconds
      barWidth
      sevenDayThreshold
      tokenFormat
      ;
    widgets = {
      inherit (cfg.widgets)
        row1
        row2
        row3
        row4
        hide
        ;
    };
  }
)
