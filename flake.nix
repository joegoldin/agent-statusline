{
  description = "Statusline for terminal coding agents (Claude Code and pi)";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
        "x86_64-darwin"
      ];
      forAllSystems =
        f: nixpkgs.lib.genAttrs systems (system: f { pkgs = import nixpkgs { inherit system; }; });
    in
    {
      packages = forAllSystems (
        { pkgs }:
        rec {
          agent-statusline = pkgs.callPackage ./package.nix { };
          default = agent-statusline;

          # The pi extension. pi loads .ts directly, so this is a plain copy
          # rather than a build. piEntrypoint is what pi-nix hands to
          # --extension; keeping it in passthru means consumers never have to
          # know the file layout.
          pi-extension =
            (pkgs.runCommand "agent-statusline-pi-extension" { } ''
              mkdir -p $out
              cp ${./extension/statusline.ts} $out/statusline.ts
              cp ${./extension/package.json} $out/package.json
            '').overrideAttrs
              (old: {
                passthru = (old.passthru or { }) // {
                  piEntrypoint = "statusline.ts";
                };
              });
        }
      );

      lib = forAllSystems ({ pkgs }: import ./lib { inherit pkgs; });

      checks = forAllSystems (
        { pkgs }:
        {
          options-tests = import ./tests/options-test.nix { inherit pkgs; };

          agent-statusline-tests =
            pkgs.runCommand "agent-statusline-tests"
              {
                nativeBuildInputs = [ pkgs.go ];
                src = ./.;
              }
              ''
                cp -r $src work && chmod -R u+w work && cd work
                export HOME=$TMPDIR GOCACHE=$TMPDIR/go-cache GOFLAGS=-mod=vendor
                go test ./...
                touch $out
              '';
        }
      );

      formatter = forAllSystems ({ pkgs }: pkgs.nixfmt);
    };
}
