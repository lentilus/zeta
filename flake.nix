{ description = "zeta";

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
  system = "x86_64-linux";
  pkgs   = import nixpkgs { inherit system; };

  zeta = pkgs.buildGoModule {
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
  };

  debugCmd = pkgs.writeShellScriptBin "debug" ''
    rm -rf /tmp/zeta-testing/*
    mkdir -p /tmp/zeta-test-notes
    mkdir -p /tmp/zeta-testing
    go build -o /tmp/zeta-testing/zeta -gcflags=all=-N ./cmd/zeta || exit
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
    packages.${system} = {
      default = zeta;
      zeta    = zeta;
    };

    devShells.${system}.default = pkgs.mkShell {
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
        vendorCmd
      ];
    };
  };
}
