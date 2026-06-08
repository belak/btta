{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };
  };

  outputs =
    inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [
        "aarch64-linux"
        "x86_64-linux"
        "aarch64-darwin"
      ];

      perSystem =
        {
          config,
          lib,
          pkgs,
          ...
        }:
        {
          packages = lib.packagesFromDirectoryRecursive {
            callPackage = pkgs.callPackage;
            directory = ./nix/pkgs;
          };

          devShells.default = pkgs.mkShell {
            env.GOTOOLCHAIN = "local";

            packages = with pkgs; [
              go
              gopls
              gotools
              config.packages.sqlc
              config.packages.templ

              just
              mprocs
              treefmt
              nixfmt
              watchexec

              nodejs
              prettier
              pnpm
            ];
          };
        };
    };
}
