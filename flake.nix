{
  description = "Barebones static Website generator from flavored Markdown";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = inputs @ {flake-parts, ...}:
    flake-parts.lib.mkFlake {inherit inputs;} {
      imports = [];
      systems = ["x86_64-linux" "aarch64-darwin"];
      perSystem = {
        config,
        self',
        inputs',
        pkgs,
        system,
        ...
      }: {
        devShells.default = pkgs.mkShell {
          name = "nyx";
          packages = with pkgs; [
            nil # nix ls
            alejandra # formatter
            git # flakes require git, and so do I
            glow # markdown viewer
            nodePackages.serve # serve static content over http
          ];
        };

        # provide the formatter for nix fmt
        formatter = pkgs.alejandra;
      };
      flake = {
        # The usual flake attributes can be defined here, including system-
        # agnostic ones like nixosModule and system-enumerating ones, although
        # those are more easily expressed in perSystem.
      };
    };
}
