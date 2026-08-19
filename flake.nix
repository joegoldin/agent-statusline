{
  description = "Statusline for terminal coding agents (Claude Code and pi)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

    # Pinned to the same 2.1.0 pi-nix uses, so the two flakes share a store path.
    bun2nix = {
      url = "github:nix-community/bun2nix?ref=2.1.0";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      bun2nix,
    }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
        "x86_64-darwin"
      ];
      forAllSystems =
        f:
        nixpkgs.lib.genAttrs systems (
          system:
          f {
            pkgs = import nixpkgs { inherit system; };
            bunPkgs = import nixpkgs {
              inherit system;
              overlays = [ bun2nix.overlays.default ];
            };
          }
        );
    in
    {
      packages = forAllSystems (
        { pkgs, bunPkgs }:
        rec {
          agent-statusline = pkgs.callPackage ./package.nix { };
          default = agent-statusline;

          # The pi extension. pi loads .ts directly, so this is a plain copy
          # rather than a build. piEntrypoint is what pi-nix hands to
          # --extension; keeping it in passthru means consumers never have to
          # know the file layout.
          pi-extension =
            (pkgs.runCommand "agent-statusline-pi-extension" { } ''
              mkdir -p $out/src
              cp ${./extension/statusline.ts} $out/statusline.ts
              cp ${./extension/package.json} $out/package.json
              cp ${./extension/src}/*.ts $out/src/
              # Tests and the theme double never ship: they import
              # devDependencies that will not exist beside the installed file.
              rm -f $out/src/*.test.ts $out/src/testing.ts
              test ! -e $out/src/testing.ts
              # And nothing that ships may import a bare specifier: pi installs
              # these files with no node_modules beside them, so an import that
              # is neither relative nor node: fails at load rather than at build.
              if grep -rho 'from "[^"]*"' $out/src $out/statusline.ts \
                | grep -v 'from "\.' | grep -v 'from "node:'; then
                echo "the shipped extension imports a bare specifier (above)" >&2
                exit 1
              fi
            '').overrideAttrs
              (old: {
                passthru = (old.passthru or { }) // {
                  piEntrypoint = "statusline.ts";
                };
              });
        }
      );

      lib = forAllSystems ({ pkgs, ... }: import ./lib { inherit pkgs; });

      checks = forAllSystems (
        { pkgs, bunPkgs }:
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

          pi-extension-tests = bunPkgs.stdenv.mkDerivation {
            name = "agent-statusline-pi-extension-tests";
            src = ./extension;

            nativeBuildInputs = [
              bunPkgs.bun2nix.hook
              bunPkgs.bun
            ];

            bunDeps = bunPkgs.bun2nix.fetchBunDeps {
              bunNix = import ./extension/bun.nix;
            };

            # pi-coding-agent's postinstall fetches a binary; the tests only
            # read TypeScript and JavaScript out of the package tree.
            dontRunLifecycleScripts = true;

            # bun2nix's hook adds a `bun build` buildPhase for applications.
            # There is nothing to bundle here: pi loads the .ts sources
            # directly, and this derivation exists only to run the tests.
            dontBuild = true;

            # The Go binary's real --emit json output is the fixture, and it is
            # checked in; the tests never spawn it, so no Go toolchain is needed.
            checkPhase = ''
              runHook preCheck
              export HOME=$TMPDIR
              bun test
              runHook postCheck
            '';
            doCheck = true;

            installPhase = "touch $out";
          };
        }
      );

      formatter = forAllSystems ({ pkgs, ... }: pkgs.nixfmt);
    };
}
