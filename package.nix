{
  lib,
  buildGoModule,
  makeWrapper,
  jj-starship,
}:
buildGoModule {
  pname = "agent-statusline";
  version = "0.2.0";

  src = lib.cleanSource ./.;

  vendorHash = null; # using vendored deps

  subPackages = [ "cmd/agent-statusline" ];

  ldflags = [
    "-s"
    "-w"
  ];

  nativeBuildInputs = [ makeWrapper ];

  # The git widget renders through jj-starship and falls back to plain
  # `git status` when it is missing, so it is suffixed: a user's own copy wins.
  postInstall = ''
    wrapProgram $out/bin/agent-statusline \
      --suffix PATH : ${lib.makeBinPath [ jj-starship ]}
  '';

  meta = with lib; {
    description = "Statusline for terminal coding agents (Claude Code and pi)";
    license = licenses.mit;
    mainProgram = "agent-statusline";
    platforms = platforms.unix;
  };
}
