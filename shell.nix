{
  pkgs ?
    import (fetchTarball
      # go: v1.25.5
      "https://github.com/NixOS/nixpkgs/archive/a1bab9e494f5f4939442a57a58d0449a109593fe.tar.gz")
    {},
}: let
  helpers = import (builtins.fetchTarball
    "https://github.com/loicsikidi/nix-shell-toolbox/tarball/main") {
    inherit pkgs;
    hooksConfig = {
      treefmt.enable = true;
    };
  };
in
  pkgs.mkShell {
    buildInputs = with pkgs;
      [
        goreleaser
        cosign
        syft
        gcc
        grpcurl
      ]
      ++ helpers.packages;

    shellHook = ''
      ${helpers.shellHook}
      echo "Development environment ready!"
      echo "  - Go version: $(go version)"
    '';

    # to enable debugging with delve
    hardeningDisable = ["fortify"];

    env = {
      CGO_ENABLED = "1";
    };
  }
