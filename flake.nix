{
  description = "zeta";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        deps =  (final: prev: {
          force-graph = prev.fetchurl {
            url = "https://cdn.jsdelivr.net/npm/force-graph@1.49.5/dist/force-graph.min.js";
            sha256 = "sha256-x3jy78zXsY6aQDD1PYHTGfF5qKuPvG8QAB3GyQTSA6E=";
          };

          tree-sitter-typst = prev.fetchFromGitHub {
            owner = "uben0";
            repo = "tree-sitter-typst";
            rev = "46cf4ded12ee974a70bf8457263b67ad7ee0379d";
            sha256 = "sha256-s/9R3DKA6dix6BkU4mGXaVggE4bnzOyu20T1wuqHQxk=";
          };

          go-llama-src = prev.fetchFromGitHub {
            owner = "go-skynet";
            repo = "go-llama.cpp";
            rev = "6a8041ef6b46d4712afc3ae791d1c2d73da0ad1c";
            sha256 = "sha256-IcgyWLnzvNQ0QRCJXhME8j/95PuyGlDjF6cxT2k5gIE=";
            fetchSubmodules = true;
          };
        });

        pkgs = import nixpkgs {
          inherit system;
          overlays  = [ deps ];
        };

        # MUSL variant of the same package-set — we build the static binary with this
        mpkgs = pkgs.pkgsMusl;

        # Prebuild the go-llama.cpp binding library as a separate derivation.
        # This produces $libgo-llama/lib/libbinding.a and $libgo-llama/include/...
        libgo-llama = mpkgs.stdenv.mkDerivation rec {
          pname = "go-llama-binding";
          version = "0.1";
          src = pkgs.go-llama-src;

          nativeBuildInputs = [ mpkgs.gcc mpkgs.gnumake mpkgs.cmake ];

          configurePhase = ''
            # nothing to do
          '';

          buildPhase = ''
            # build the static binding
            make libbinding.a
          '';

          installPhase = ''
            ls -al
            exit 1
          '';
        };

      in {
        packages.default = mpkgs.buildGoModule rec {
          pname   = "zeta";
          version = "0.3.6";
          src     = ./.;

          buildInputs = [ mpkgs.go mpkgs.gcc ];
          nativeBuildInputs = [ mpkgs.musl ];

          # Use the prebuilt binding store path for both library and include.
          env = {
            CGO_ENABLED = "1";
            CC = "${mpkgs.stdenv.cc}";
            LIBRARY_PATH = "${libgo-llama}/lib";
            C_INCLUDE_PATH = "${libgo-llama}/include";
            # If linking errors happen in the static binary, you may need to add:
            # CGO_LDFLAGS = "-L${libgo-llama}/lib -lbinding -lstdc++"
          };

          # static link against musl
          ldflags = [
            "-s" "-w"
            "-linkmode external"
            "-extldflags '-static -L${mpkgs.musl}/lib'"
            "-X main.Version=v${version}"
          ];

          vendorHash = "sha256-6muGhy8MNOC5EkFtoGCQ3QgEMKYsg0Y/aG2HBJsJqnM=";
          doCheck    = false;

          patchPhase = ''
            mkdir -p external/_vendor
            rm -rf .gitignore
            cp -r ${pkgs.tree-sitter-typst} external/_vendor/tree-sitter-typst
            cp -r ${pkgs.force-graph}   external/_vendor/force-graph.js

            # create a tiny helper dir so code requiring a local external/go-llama.cpp path still works
            mkdir -p external/go-llama.cpp
            echo "Using prebuilt go-llama binding from: ${libgo-llama}" > external/go-llama.cpp/README
          '';
        };

        devShells.default = pkgs.mkShell {
          shellHook = ''
            echo "== Welcome to zeta dev shell =="
          '';

          buildInputs = with pkgs; [
            go
            gopls
            gofumpt
            gotools
            golines
            typst
            tinymist
            pv
            # helper commands that live in the original flake
            (writeShellScriptBin "debug" ''
              rm -rf /tmp/zeta-testing/*
              mkdir -p /tmp/zeta-test-notes
              mkdir -p /tmp/zeta-testing

              # Ensure we point to the prebuilt binding library in the store
              export LIBBINDING_LIB="${libgo-llama}/lib/libbinding.a"
              export LIBBINDING_LIB_DIR="${libgo-llama}/lib"
              export LIBBINDING_INCLUDE_DIR="${libgo-llama}/include"

              if [ ! -f "${libgo-llama}/lib/libbinding.a" ]; then
                echo "Prebuilt libbinding.a not found at ${libgo-llama}/lib/libbinding.a"
                echo "Try running: nix build .#libgo-llama"
                exit 1
              fi

              # Build the go binary for debugging; the buildGoModule will already use the same store path.
              CGO_ENABLED=1 CC="${mpkgs.stdenv.cc}" \
                LIBRARY_PATH="${libgo-llama}/lib" C_INCLUDE_PATH="${libgo-llama}/include" \
                go build -o /tmp/zeta-testing/zeta -gcflags=all=-N . || exit

              PATH="/tmp/zeta-testing:$PATH"
              exec ${neovim}/bin/nvim -u ${./_example/init.lua} /tmp/zeta-test-notes/test.typ
            '')
            (writeShellScriptBin "debugRelease" ''
              rm -rf /tmp/zeta-testing/*
              mkdir -p /tmp/zeta-test-notes
              mkdir -p /tmp/zeta-testing
              nix build .#zeta || exit
              cp result/bin/zeta /tmp/zeta-testing/zeta
              PATH="/tmp/zeta-testing:$PATH"
              exec ${neovim}/bin/nvim -u ${./_example/init.lua} /tmp/zeta-test-notes/test.typ
            '')
            (writeShellScriptBin "vendor" ''
              echo "Populating _vendor directory..."
              rm -rf external/_vendor
              mkdir -p external/_vendor
              cp -r --no-preserve=mode,ownership ${pkgs.tree-sitter-typst} external/_vendor/tree-sitter-typst
              cp -r --no-preserve=mode,ownership ${pkgs.force-graph} external/_vendor/force-graph.js

              echo "Using prebuilt go-llama binding from ${libgo-llama}"
              # Optionally expose a copy for local editing (not required)
              rm -rf external/go-llama.cpp
              mkdir -p external/go-llama.cpp
              cp -r ${libgo-llama}/include external/go-llama.cpp/include || true
              cp -r ${libgo-llama}/lib external/go-llama.cpp/lib || true

              echo "_vendor directory is now up to date."
            '')
            (writeShellScriptBin "demo" ''
              rm -rf /tmp/zeta-demo-notes
              mkdir -p /tmp/zeta-demo-notes
              cd /tmp/zeta-demo-notes

              pv -qL 20 ${./_example/demo.txt} \
                | script -q -c \
                "stty rows $(tput lines) cols $(tput cols); \
                nvim -u ${./_example/demo.lua}" \
                /dev/null
            '')
          ];

          # bring neovim into the shell for the scripts above
          nativeBuildInputs = with pkgs; [ neovim ];
        };
      }
    );
}
