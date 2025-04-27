{
  description = "zeta";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

    tree-sitter-typst = {
      url = "github:uben0/tree-sitter-typst";
      flake = false;
    };

    d3 = {
      url = "https://d3js.org/d3.v5.min.js";
      flake = false;
    };

    forcegraph = {
      url = "https://cdn.jsdelivr.net/npm/force-graph";
      flake = false;
    };
  };

  outputs = { self, nixpkgs, ... }@inputs: let
    systems = [
      "x86_64-linux"
      "aarch64-linux"
      "x86_64-darwin"
      "aarch64-darwin"
    ];

    forAllSystems = f: nixpkgs.lib.genAttrs systems (system:
      f {
        inherit system;
        pkgs = import nixpkgs { inherit system; };
      }
    );
  in {
    packages = forAllSystems ({ pkgs, system }: let
      zeta = pkgs.buildGoModule rec {
        pname       = "zeta";
        version     = "0.3.0";
        src         = ./.;
        buildInputs = [ pkgs.go ];
        vendorHash  = "sha256-MR40dtOpVQ8MCAEDiwl1S2rz/HAvfpcaRiTdy/irOVA=";

        doCheck = false;
        patchPhase = ''
          mkdir -p external/_vendor
          rm -rf .gitignore
          cp -r ${inputs.tree-sitter-typst} external/_vendor/tree-sitter-typst
          cp -r ${inputs.forcegraph} external/_vendor/force-graph.js
          cp ${inputs.d3} external/_vendor/d3.v5.min.js
        '';

        ldflags = [
          "-s" "-w" # minimize bin size
          "-X main.Version=v${version}" # inject version
        ];
      };
    in {
      default = zeta;
      zeta = zeta;
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
        cp -r --no-preserve=mode,ownership ${inputs.tree-sitter-typst} external/_vendor/tree-sitter-typst
        cp -r --no-preserve=mode,ownership ${inputs.forcegraph} external/_vendor/force-graph.js
        cp --no-preserve=mode,ownership ${inputs.d3} external/_vendor/d3.v5.min.js
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
