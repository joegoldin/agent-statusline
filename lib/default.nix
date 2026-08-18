# The shared statusline schema, consumed by claude-nix and pi-nix so both
# harnesses configure one binary through one set of options.
#
#   statuslineOptions : attrset of mkOptions, mountable under any namespace
#   renderConfig      : evaluated options -> writeText derivation holding the
#                       config JSON the binary reads
{
  pkgs,
  lib ? pkgs.lib,
}:
{
  statuslineOptions = import ./options.nix { inherit lib; };
  renderConfig = import ./render-config.nix { inherit pkgs lib; };
}
