# The agent-statusline configuration schema, as a bare attrset of `mkOption`s.
#
# It is deliberately *not* a module: consumers (claude-nix, pi-nix) mount it
# under whatever namespace suits them, e.g.
#
#   statusLine = mkOption {
#     type = types.submodule { options = agent-statusline.lib.statuslineOptions; };
#     default = { };
#   };
#
# Every option whose name matches a key of Go's `config.Config`
# (internal/config/config.go) must keep its default in step with that struct's
# `Defaults()`. `tests/options-test.nix` is the gate that enforces it.
#
# `enable`, `package`, `hideVimModeIndicator` and `toolTiming` are harness-level
# concerns — the binary never reads them, so `renderConfig` drops them.
{ lib }:
let
  inherit (lib) mkOption types;

  # Every widget the binary can place on a row. Derived from the registry
  # built in cmd/agent-statusline/main.go (`buildRegistry`) — i.e. each
  # widget's `Name()` — and cross-checked against `dropPriority` in the same
  # file, which lists exactly the same seventeen names.
  #
  # The activity-stack widgets (tools, agents, todos) are
  # deliberately absent: they are a fixed, non-configurable stack sized by
  # `activityRows`, they are not in the registry, and `widgets.hide` is only
  # consulted by the row layout, never by the activity stack.
  widgetNames = [
    "model"
    "cwd"
    "git"
    "duration"
    "usage5h"
    "usage7d"
    "context"
    "tokens"
    "burnRate"
    "effort"
    "voice"
    "compaction"
    "pr"
    "cost"
    "sessionName"
    "autoMode"
    "cache"
  ];
in
{
  enable = lib.mkEnableOption "the agent-statusline binary";

  package = mkOption {
    type = types.package;
    description = ''
      The agent-statusline binary to install. Has no default: the consuming
      module supplies it, normally
      `agent-statusline.packages.''${system}.agent-statusline`.
    '';
  };

  padding = mkOption {
    type = types.int;
    default = 0;
    description = "Horizontal padding cells, passed to the harness's statusLine.padding.";
  };

  hideVimModeIndicator = mkOption {
    type = types.nullOr types.bool;
    default = null;
    description = ''
      Hide Claude Code's built-in `-- INSERT --` / `-- VISUAL --`
      line below the prompt (`statusLine.hideVimModeIndicator`).
      Only worth setting when `editorMode = "vim"` and your status
      line renders the mode itself. Null (default) omits the key.

      Claude Code only; the binary never reads it and other harnesses ignore
      it.
    '';
  };

  refreshInterval = mkOption {
    type = types.int;
    default = 1;
    description = ''
      Seconds between forced re-renders, in addition to the harness's
      event-driven updates (0 = event-driven only). Defaults
      to 1 so time-based segments — session clock, burn-rate ETA,
      rate-limit reset countdowns, agent elapsed — tick live. The
      binary caches the expensive work (git porcelain via TTL, the
      parsed transcript via mtime), so a 1s cadence is cheap.
    '';
  };

  activityRows = mkOption {
    type = types.ints.between 0 4;
    default = 4;
    description = ''
      Maximum number of activity rows to render. The activity stack
      (in order) is: running tools, recent-tool counts, agents, todos.
      Each row hides when empty.
    '';
  };

  hideWhenIdle = mkOption {
    type = types.bool;
    default = true;
    description = "Hide activity rows entirely when there is no recent activity.";
  };

  widgets = mkOption {
    description = "Ordered widget lists per row plus a universal hide list.";
    default = { };
    type = types.submodule {
      options = {
        row1 = mkOption {
          type = types.listOf (types.enum widgetNames);
          default = [
            "model"
            "cwd"
            "git"
            "duration"
            "usage5h"
            "usage7d"
          ];
          description = ''
            Top row — identity, session clock & account usage. The
            model widget appends the current effort inline (e.g.
            "Opus 4.7 xhigh"); duration sits right after git.
          '';
        };
        row2 = mkOption {
          type = types.listOf (types.enum widgetNames);
          default = [
            "context"
            "tokens"
            "burnRate"
            "voice"
            "compaction"
            "pr"
            "cost"
          ];
          description = "Bottom row — this conversation's state.";
        };
        row3 = mkOption {
          type = types.listOf (types.enum widgetNames);
          default = [ "autoMode" ];
          description = ''
            Third row — the pi-automode permission tally. Rendered on a
            line of its own because the widget draws several figures.
            Empty to switch the row off; it hides itself anyway unless
            pi-automode is publishing its status text, which it only does
            with `PI_AUTOMODE_NO_STATUS_SLOT = "1"` in the environment.
          '';
        };
        row4 = mkOption {
          type = types.listOf (types.enum widgetNames);
          default = [ "cache" ];
          description = ''
            Fourth row — prompt-cache hit rate for the active model, read
            from the pi cache-optimizer extension's sidecar in
            `$PI_CODING_AGENT_DIR`. Empty to switch the row off; it hides
            itself when that extension has recorded nothing for the model.
          '';
        };
        hide = mkOption {
          type = types.listOf (types.enum widgetNames);
          default = [ ];
          description = "Widgets to suppress everywhere.";
        };
      };
    };
  };

  gitCacheTtlSeconds = mkOption {
    type = types.int;
    default = 5;
    description = "Git porcelain cache TTL in seconds.";
  };

  transcriptWindowSeconds = mkOption {
    type = types.int;
    default = 300;
    description = ''
      Time constant (τ) for the burn-rate EMA, in seconds. Larger
      values produce a more stable display that's less reactive to
      individual file-read spikes, at the cost of taking longer
      (~3τ) to converge on a sustained rate change. Default 300s
      (5 min) is a smooth-but-still-responsive middle ground.
    '';
  };

  barWidth = mkOption {
    type = types.int;
    default = 10;
    description = ''
      Width in cells of progress bars.

      Tracks Go's `config.Defaults()` (10), which `internal/config` pins with
      its own unit test. `claude-nix` historically shipped 8 here; a consumer
      that wants the narrower bar sets `barWidth = 8` in its own config rather
      than moving this default, which would break the Nix/Go drift check.
    '';
  };

  sevenDayThreshold = mkOption {
    type = types.int;
    default = 50;
    description = "Only render usage7d once usage crosses this percent.";
  };

  tokenFormat = mkOption {
    type = types.enum [
      "compact"
      "raw"
    ];
    default = "compact";
    description = "Token count format: compact (516.9k / 1.2M tokens) or raw (516987 tokens).";
  };

  toolTiming = mkOption {
    type = types.bool;
    default = true;
    description = ''
      Register PermissionRequest / PostToolUse / PostToolUseFailure
      hooks (pointing at the same statusline binary, run as
      `agent-statusline hook`) that record each tool's real
      execution start/end to a per-session sidecar.

      The transcript only records when a tool_use is emitted and when
      its result lands — never when it actually starts — and the
      "Waiting…" (queued / awaiting-permission) state is never written
      to disk. With these hooks the running-tools row shows an
      hourglass for a tool that's emitted but not yet started, a
      spinner with elapsed measured from the real start (excluding
      queue + permission wait) once it runs, and a correct final run
      length when it finishes. Without them the row still works,
      falling back to emission-relative elapsed.

      The hooks are additive: they concatenate with any
      `extraHooks` / `settings.hooks` entries for the same events.
    '';
  };
}
