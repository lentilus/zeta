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
      packages.${system}.default = pkgs.buildGoModule {
       name = "zeta";
       src = ./.;
       vendorHash = "sha256-MR40dtOpVQ8MCAEDiwl1S2rz/HAvfpcaRiTdy/irOVA=";
      };
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
