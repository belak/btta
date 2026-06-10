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
        let
          # pnpm 11 (the nixpkgs default behind fetchPnpmDeps) crashes on
          # exit under node 24, and the frontend lockfile was generated with
          # pnpm 10 — pin it for both the deps fetcher and the devshell.
          pnpm = pkgs.pnpm_10;

          version = "0.1.0";
        in
        {
          packages =
            lib.packagesFromDirectoryRecursive {
              callPackage = pkgs.callPackage;
              directory = ./nix/pkgs;
            }
            // {
              default = config.packages.btta;

              # The Svelte frontend, built by Vite into a directory that
              # cmd/btta embeds via go:embed.
              btta-frontend = pkgs.stdenv.mkDerivation (finalAttrs: {
                pname = "btta-frontend";
                inherit version;

                src = ./frontend;

                pnpmDeps = (pkgs.fetchPnpmDeps.override { inherit pnpm; }) {
                  inherit (finalAttrs) pname version src;
                  fetcherVersion = 3;
                  hash = "sha256-U3r6BkEM6z5ErpHyhpYsq/JsGK0beyJr6cu+Jj+ezT8=";
                };

                nativeBuildInputs = [
                  pkgs.nodejs
                  pnpm
                  (pkgs.pnpmConfigHook.override { inherit pnpm; })
                ];

                buildPhase = ''
                  runHook preBuild
                  pnpm exec vite build --outDir dist --emptyOutDir
                  runHook postBuild
                '';

                installPhase = ''
                  runHook preInstall
                  cp -r dist $out
                  runHook postInstall
                '';
              });

              btta = pkgs.buildGoModule {
                pname = "btta";
                inherit version;

                src = ./.;

                vendorHash = "sha256-cuqudbYbINka79axVcUbL6EYAZm01rqLfILmUrsjBpM=";

                subPackages = [ "cmd/btta" ];

                env.CGO_ENABLED = 0;

                # Drop the prebuilt frontend into the embed directory before
                # the Go build runs go:embed over it.
                preBuild = ''
                  cp -r ${config.packages.btta-frontend}/. internal/http/frontend/dist/
                '';

                ldflags = [
                  "-s"
                  "-w"
                  "-X github.com/belak/btta/internal/buildinfo.Version=${version}"
                ];
              };
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
