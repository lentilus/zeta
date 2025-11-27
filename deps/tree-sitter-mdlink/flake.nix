{
  description = "Minimal Treesitter grammar to detect markdown links";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs, ... }:
  let
    system = "x86_64-linux";
    pkgs = import nixpkgs { inherit system; };
  in
  {
    devShells."${system}".default = pkgs.mkShell {
      # only the minimal tools
      buildInputs = with pkgs; [
        tree-sitter
        nodejs
      ];

      shellHook = ''
        echo "== minimal tree-sitter dev shell =="
        if command -v tree-sitter >/dev/null 2>&1; then
          tree-sitter --version || echo "tree-sitter present (version unknown)"
        else
          echo "tree-sitter: not found"
        fi
        echo ""
        echo "Usage examples:"
        echo "  # generate / inspect a grammar (if your repo contains a grammar)"
        echo "  tree-sitter generate   # (if your grammar repo expects the CLI binary)"
        echo "  tree-sitter parse -q <file>"
      '';
    };
  };
}

