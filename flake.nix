{
  description = "zeta";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs, ... }@inputs:
  let
    systems = [ "x86_64-linux" "aarch64-linux" ];

    # overlay dependencies
    overlay = final: prev: {
      force-graph = prev.fetchurl {
        url = "https://cdn.jsdelivr.net/npm/force-graph@1.49.5/dist/force-graph.min.js";
        sha256 = "sha256-x3jy78zXsY6aQDD1PYHTGfF5qKuPvG8QAB3GyQTSA6E=";
      };
      d3 = prev.fetchurl {
        url = "https://cdn.jsdelivr.net/npm/d3@7.9.0/dist/d3.min.js";
        sha256 = "sha256-8glLv2FBs1lyLE/kVOtsSw8OQswQzHr5IfwVj864ZTk=";
      };
      tree-sitter-typst = prev.fetchFromGitHub {
        owner = "uben0";
        repo = "tree-sitter-typst";
        rev = "46cf4ded12ee974a70bf8457263b67ad7ee0379d";
        sha256 = "sha256-s/9R3DKA6dix6BkU4mGXaVggE4bnzOyu20T1wuqHQxk=";
      };
    };

    # Helper to import nixpkgs with our overlay
    forAllSystems = f:
      nixpkgs.lib.genAttrs systems (system:
        let
          pkgs = import nixpkgs {
            inherit system;
            overlays = [ overlay ];
          };
        in f { inherit pkgs system; }
      );
  in {
    packages = forAllSystems ({ pkgs, system }: rec {
      zeta = pkgs.buildGoModule rec {
        pname   = "zeta";
        version = "0.3.0";
        src     = ./.;

        buildInputs = [
          pkgs.go
          pkgs.gcc
          pkgs.glibc.static
          pkgs.glibcLocales
        ];

        env.CGO_ENABLED = "1";

        ldflags = [
          "-s" "-w"
          "-linkmode external"
          "-extldflags -static"
          "-X main.Version=v${version}"
        ];

        vendorHash = "sha256-6muGhy8MNOC5EkFtoGCQ3QgEMKYsg0Y/aG2HBJsJqnM=";
        doCheck    = false;

        patchPhase = ''
          mkdir -p external/_vendor
          rm -rf .gitignore
          cp -r ${pkgs.tree-sitter-typst} external/_vendor/tree-sitter-typst
          cp -r ${pkgs.force-graph}   external/_vendor/force-graph.js
          cp    ${pkgs.d3}           external/_vendor/d3.v5.min.js
        '';
      };

      default = zeta;
    });

devShells = forAllSystems ({ pkgs, system }: let
      debugCmd = pkgs.writeShellScriptBin "debug" ''
        rm -rf /tmp/zeta-testing/*
        mkdir -p /tmp/zeta-test-notes
        mkdir -p /tmp/zeta-testing
        go build -o /tmp/zeta-testing/zeta -gcflags=all=-N . || exit
        PATH="/tmp/zeta-testing:$PATH"
        exec ${pkgs.neovim}/bin/nvim -u ${./_example/init.lua} /tmp/zeta-test-notes/test.typ
      '';

      debugReleaseCmd = pkgs.writeShellScriptBin "debugRelease" ''
        rm -rf /tmp/zeta-testing/*
        mkdir -p /tmp/zeta-test-notes
        mkdir -p /tmp/zeta-testing
        nix build .#zeta || exit
        cp result/bin/zeta /tmp/zeta-testing/zeta
        PATH="/tmp/zeta-testing:$PATH"
        exec ${pkgs.neovim}/bin/nvim -u ${./_example/init.lua} /tmp/zeta-test-notes/test.typ
      '';

      vendorCmd = pkgs.writeShellScriptBin "vendor" ''
        echo "Populating _vendor directory..."
        rm -rf external/_vendor
        mkdir -p external/_vendor
        cp -r --no-preserve=mode,ownership ${pkgs.tree-sitter-typst} external/_vendor/tree-sitter-typst
        cp -r --no-preserve=mode,ownership ${pkgs.force-graph} external/_vendor/force-graph.js
        cp --no-preserve=mode,ownership ${pkgs.d3} external/_vendor/d3.v5.min.js
        echo "_vendor directory is now up to date."
      '';
    in {
      default = pkgs.mkShell {
        shellHook = ''
          echo "== Welcome to zeta dev shell =="
        '';
        buildInputs = [
          pkgs.go
          pkgs.gopls
          pkgs.gofumpt
          pkgs.gotools
          pkgs.golines
          pkgs.typst
          pkgs.tinymist
          debugCmd
          debugReleaseCmd
          vendorCmd
        ];
      };
    });
  };
}
