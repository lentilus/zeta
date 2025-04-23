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

vim.api.nvim_create_autocmd({ "BufReadPost", "BufNewFile" }, {
  pattern = "*.typ",
  callback = function()
    vim.lsp.start {
      name = "zeta",
      cmd = { "/tmp/zeta-dev" },
      filetypes = { "typst" },
      root_dir = "/tmp/zeta-test-notes",
      capabilities = vim.lsp.protocol.make_client_capabilities(),
      single_file_support = true,
      init_options = {
        query = '(code (call item: (ident) @link (#eq? @link "link") (group (string) @target )))',
        select_regex = '^\"(.*)\"$',
      },
      on_attach = function(client, bufnr)
        print("LSP attached to buffer", bufnr)

        local function buf_set_keymap(...)
          vim.api.nvim_buf_set_keymap(bufnr, ...)
        end
        local opts = { noremap=true, silent=true }

        -- Keybindings for common LSP features
        buf_set_keymap('n', 'gd', '<cmd>lua vim.lsp.buf.definition()<CR>', opts)
        buf_set_keymap('n', 'gr', '<cmd>lua vim.lsp.buf.references()<CR>', opts)

        -- Define :ZetaGraph command as you already have
        vim.api.nvim_buf_create_user_command(bufnr, "ZetaGraph", function()
          client.request(
            "workspace/executeCommand",
            { command = "graph", arguments = {} },
            function(err, result)
              if err then
                vim.notify("Error executing graph command: " .. err.message, vim.log.levels.ERROR)
              else
                vim.notify("Graph command executed.")
              end
            end,
            bufnr
          )
        end, { desc = "Execute Zeta LSP 'graph' command" })
      end
    }
  end,
})
        '';
      in pkgs.writeShellScriptBin "debug" ''
        rm -rf /tmp/zeta-dev
        mkdir -p /tmp/zeta-test-notes
        go build -o /tmp/zeta-dev -gcflags=all=-N ./cmd/zeta || exit
        exec ${pkgs.neovim}/bin/nvim -u ${config} /tmp/zeta-test-notes/test.typ
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
