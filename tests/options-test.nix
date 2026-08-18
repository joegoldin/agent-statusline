# Evaluates the shared options with defaults and asserts the rendered config
# JSON matches the Go side's compiled-in defaults (internal/config/config.go
# `Defaults()`). If these drift, the Nix defaults silently stop matching Go's
# and a user who never overrides a key gets one value from a fresh checkout and
# another from a Nix-managed one.
#
# `expected` below is a field-for-field transcription of `config.Defaults()`.
# When you change a default in Go, change it here and in lib/options.nix; when
# you add a field to config.Config, add it in all three places. Go is the
# reference for rendering behaviour, so Go wins any disagreement.
#
# The comparison reads `.text` off the writeText derivation rather than
# `builtins.readFile`, so the check stays a pure evaluation with no
# import-from-derivation.
{
  pkgs ? import <nixpkgs> { },
}:
let
  lib = pkgs.lib;
  statuslineLib = import ../lib { inherit pkgs lib; };

  evaluated =
    (lib.evalModules {
      modules = [
        {
          options.statusLine = lib.mkOption {
            type = lib.types.submodule { options = statuslineLib.statuslineOptions; };
            default = { };
          };
        }
      ];
    }).config.statusLine;

  rendered = builtins.fromJSON (statuslineLib.renderConfig evaluated).text;

  # internal/config/config.go, func Defaults().
  expected = {
    padding = 0;
    refreshInterval = 1;
    activityRows = 4;
    hideWhenIdle = true;
    widgets = {
      row1 = [
        "model"
        "cwd"
        "git"
        "duration"
        "usage5h"
        "usage7d"
      ];
      row2 = [
        "context"
        "tokens"
        "burnRate"
        "voice"
        "compaction"
        "pr"
        "cost"
      ];
      hide = [ ];
    };
    gitCacheTtlSeconds = 5;
    transcriptWindowSeconds = 300;
    barWidth = 10;
    sevenDayThreshold = 50;
    tokenFormat = "compact";
  };

  # Named per-key diff, so a failure says which field drifted instead of just
  # "assertion failed".
  keys = lib.unique (builtins.attrNames expected ++ builtins.attrNames rendered);
  mismatches = builtins.filter (
    k: !(builtins.hasAttr k rendered && builtins.hasAttr k expected && rendered.${k} == expected.${k})
  ) keys;
in
assert lib.assertMsg (mismatches == [ ]) ''
  Nix statusline defaults have drifted from Go's config.Defaults().
  Mismatched keys: ${builtins.concatStringsSep ", " mismatches}
    rendered: ${builtins.toJSON (lib.filterAttrs (k: _: builtins.elem k mismatches) rendered)}
    expected: ${builtins.toJSON (lib.filterAttrs (k: _: builtins.elem k mismatches) expected)}
'';
# A widget name the registry does not know must not typecheck.
assert
  !(builtins.tryEval (
    lib.deepSeq
      (lib.evalModules {
        modules = [
          {
            options.statusLine = lib.mkOption {
              type = lib.types.submodule { options = statuslineLib.statuslineOptions; };
              default = { };
            };
          }
          { statusLine.widgets.row1 = [ "notAWidget" ]; }
        ];
      }).config.statusLine.widgets.row1
      true
  )).success;
pkgs.runCommand "options-tests" { } "touch $out"
