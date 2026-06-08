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

        force-graph = pkgs.fetchurl {
          url = "https://cdn.jsdelivr.net/npm/force-graph@1.49.5/dist/force-graph.min.js";
          sha256 = "sha256-x3jy78zXsY6aQDD1PYHTGfF5qKuPvG8QAB3GyQTSA6E=";
        };

        # For reasons that are beyond me, there is a run-time error originating
        # from the grammar if we use the grammar from nixpkgs. In the long term
        # we should compile the grammar seperately and not leave CGO to mess
        # with it.
        tree-sitter-typst-src = pkgs.fetchFromGitHub {
          owner = "uben0";
          repo = "tree-sitter-typst";
          rev = "46cf4ded12ee974a70bf8457263b67ad7ee0379d";
          sha256 = "sha256-s/9R3DKA6dix6BkU4mGXaVggE4bnzOyu20T1wuqHQxk=";
        };
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
            doCheck = false;
            enableParallelBuilding = true;

            postPatch = ''
              mkdir -p external/_vendor
              rm -rf .gitignore
              cp -r ${tree-sitter-typst-src} external/_vendor/tree-sitter-typst
              cp -r ${force-graph} external/_vendor/force-graph.js
            '';
          };

          default = zeta;
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

          mkTestbed = ''
            testdir="$(mktemp -d -t zeta-testing.XXXXXX)"
            notesdir="$(mktemp -d -t zeta-test-notes.XXXXXX)"
            touch $notesdir/test.typ
            trap 'rm -rf "$testdir" "$notesdir"' EXIT
          '';

          debugCmd = pkgs.writeShellScriptBin "debug" ''
            ${mkTestbed}
            go build -o "$testdir/zeta" -gcflags=all=-N . || exit
            PATH="$testdir:$PATH"
            exec ${pkgs.neovim}/bin/nvim -u ${./_example/init.lua} "$notesdir/test.typ"
          '';

          debugReleaseCmd = pkgs.writeShellScriptBin "debugRelease" ''
            ${mkTestbed}
            PATH="${zetaPkg}/bin:$PATH"
            exec ${pkgs.neovim}/bin/nvim -u ${./_example/init.lua} "$notesdir/test.typ"
          '';

          vendorCmd = pkgs.writeShellScriptBin "vendor" ''
            echo "Populating _vendor directory..."
            rm -rf external/_vendor
            mkdir -p external/_vendor
            cp -r --no-preserve=mode,ownership ${tree-sitter-typst-src} external/_vendor/tree-sitter-typst
            cp -r --no-preserve=mode,ownership ${force-graph} external/_vendor/force-graph.js
            echo "_vendor directory is now up to date."
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
              pkgs.typst
              pkgs.tinymist
              pkgs.pv
              debugCmd
              debugReleaseCmd
              vendorCmd
              demo
            ];
          };
        };
      }
    );
}
