{
  description = "zeta";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs }: 
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in {
      devShells.${system}.default = pkgs.mkShell {
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
        ];
      };
    };
}
