{ pkgs, lib }:
# Renders evaluated statusline options into the JSON the Go binary reads
# (`internal/config`.Config, loaded from ~/.claude/statusline-config.json or
# $AGENT_STATUSLINE_CONFIG).
#
# Only the keys config.Config declares are emitted; `enable`, `package`,
# `hideVimModeIndicator` and `toolTiming` are module-level concerns and are
# deliberately excluded. Unknown keys would be tolerated by the Go decoder, but
# emitting them would make the drift check in tests/options-test.nix meaningless.
#
# `renderConfigJSON` is the pure primitive (a JSON string), so callers and tests
# can inspect the result without import-from-derivation. `renderConfig` wraps it
# in the store file consumers symlink into place.
let
  renderConfigJSON =
    cfg:
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
        inherit (cfg.widgets) row1 row2 hide;
      };
    };
in
lib.setFunctionArgs (cfg: pkgs.writeText "agent-statusline-config.json" (renderConfigJSON cfg)) {
  cfg = false;
}
// { }
