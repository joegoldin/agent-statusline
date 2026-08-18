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
        }
      );

      checks = forAllSystems (
        { pkgs }:
        {
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
