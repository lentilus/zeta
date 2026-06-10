{
  description = "zeta";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = nixpkgs.legacyPackages.${system};

        allGrammars = import ./grammars.nix {inherit pkgs;};

        mkTestbed = ''
          testdir="$(mktemp -d -t zeta-testing.XXXXXX)"
          notesdir="$(mktemp -d -t zeta-test-notes.XXXXXX)"
          touch $notesdir/test.typ
          trap 'rm -rf "$testdir" "$notesdir"' EXIT
        '';

      in {
        packages = rec {
          zeta = pkgs.buildGoModule rec {
            pname = "zeta";
            version = "0.3.7";

            src = pkgs.lib.cleanSourceWith {
              src = ./.;
              filter = path: _type: let
                base = baseNameOf path;
              in
                !(builtins.elem base ["result" "_example"]);
            };

            nativeBuildInputs = pkgs.lib.optionals pkgs.stdenv.isLinux [
              pkgs.gcc
            ];

            buildInputs = pkgs.lib.optionals pkgs.stdenv.isLinux [
              pkgs.glibc.static
            ];

            env.CGO_ENABLED = "1";

            ldflags =
              [
                "-s"
                "-w"
                "-X main.Version=v${version}"
              ]
              ++ pkgs.lib.optionals pkgs.stdenv.isLinux [
                "-linkmode external"
                "-extldflags -static"
              ];

            vendorHash = "sha256-6muGhy8MNOC5EkFtoGCQ3QgEMKYsg0Y/aG2HBJsJqnM=";
            doCheck = true;
            enableParallelBuilding = true;
          };

          default = zeta;

          vendorGrammars = pkgs.writeShellScriptBin "vendorGrammars" ''
            echo "Populating grammars directory..."
            cp -rL --no-preserve=mode,ownership ${allGrammars}/* ./grammars/
            ls -al ./grammars
          '';

          debugRelease = pkgs.writeShellScriptBin "debugRelease" ''
            ${mkTestbed}
            PATH="${zeta}/bin:$PATH"
            exec ${pkgs.neovim}/bin/nvim -u ${./_example/init.lua} "$notesdir/test.typ"
          '';
        };

        apps = {
          default = {
            type = "app";
            program = "${self.packages.${system}.zeta}/bin/zeta";
          };
        };

        formatter = pkgs.alejandra;

        devShells = let
          zetaPkg = self.packages.${system}.zeta;

          debugCmd = pkgs.writeShellScriptBin "debug" ''
            ${mkTestbed}
            go build -o "$testdir/zeta" -gcflags=all=-N . || exit
            PATH="$testdir:$PATH"
            exec ${pkgs.neovim}/bin/nvim -u ${./_example/init.lua} "$notesdir/test.typ"
          '';

          demo = pkgs.writeShellScriptBin "demo" ''
            set -e
            root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
            workdir="$(mktemp -d -t zeta-demo.XXXXXX)"
            trap 'rm -rf "$workdir"' EXIT

            go build -C "$root" -o "$workdir/zeta" .

            cd "$workdir"
            export PATH="$workdir:$PATH"
            ${pkgs.pv}/bin/pv -qL 20 "$root/_example/demo.txt" \
              | ${pkgs.expect}/bin/unbuffer -p ${pkgs.neovim}/bin/nvim -u "$root/_example/demo.lua"
          '';
        in {
          default = pkgs.mkShell {
            shellHook = ''
              echo "== Welcome to zeta dev shell =="
            '';
            packages = [
              pkgs.go
              pkgs.gopls
              pkgs.gofumpt
              pkgs.gotools
              pkgs.golines
              debugCmd
              demo
            ];
          };
        };
      }
    );
}
