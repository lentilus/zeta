{
  description = "zeta";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs }: 
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };

      zeta = pkgs.buildGoModule {
        name = "zeta";
        src = ./.;
        vendorHash = "sha256-MR40dtOpVQ8MCAEDiwl1S2rz/HAvfpcaRiTdy/irOVA=";
      };

      debug = let
        config = pkgs.writeText "init.lua" ''
          print("--DEBUG /tmp/zeta-dev --")
          vim.lsp.start {
            name = 'zeta',
            cmd = { '/tmp/zeta-dev' },
            filetypes = { 'typst' },
            root_dir = ".",
            capabilities = vim.lsp.protocol.make_client_capabilities(),
            single_file_support = true,
          }
        '';
      in pkgs.writeShellScriptBin "debug" ''
        rm -rf /tmp/zeta-dev
        go build -o /tmp/zeta-dev -gcflags=all=-N ./cmd/zeta
        exec ${pkgs.neovim}/bin/nvim -u ${config}
      '';

    in {
      packages.${system} = {
        default = zeta;
        zeta = zeta;
        debug = debug;
      };

      devShells.${system}.default = pkgs.mkShell {
        shellHook = ''
        echo "==Welcome to zeta=="
        # echo "> type `debug` for a fast dev-build + test"
        '';
        buildInputs = [
          pkgs.go
          pkgs.gopls

          # formatting
          pkgs.gofumpt
          pkgs.gotools
          pkgs.golines

          # debugging
          pkgs.typst
          pkgs.tinymist
          debug
        ];
      };
    };
}
