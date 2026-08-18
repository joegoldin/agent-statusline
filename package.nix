{
  lib,
  buildGoModule,
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

  meta = with lib; {
    description = "Statusline for terminal coding agents (Claude Code and pi)";
    license = licenses.mit;
    mainProgram = "agent-statusline";
    platforms = platforms.unix;
  };
}
