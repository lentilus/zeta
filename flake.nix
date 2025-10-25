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

          llama-cpp = prev.fetchFromGitHub {
            owner = "ggml-org";
            repo = "llama.cpp";
            rev = "5a91109a5d7dab5d7adc40bedb397ede99a705b1";
            sha256 = "sha256-4jc1Lk9Qa/S0oBDp3BbFYz35X791LYOG+/C7gyn2EXw=";
          };

          tree-sitter-typst = prev.fetchFromGitHub {
            owner = "uben0";
            repo = "tree-sitter-typst";
            rev = "46cf4ded12ee974a70bf8457263b67ad7ee0379d";
            sha256 = "sha256-s/9R3DKA6dix6BkU4mGXaVggE4bnzOyu20T1wuqHQxk=";
          };
        });

        pkgs = import nixpkgs {
          inherit system;
          overlays  = [ deps ];
        };

        # MUSL variant of the same package-set — we build the static binary with this
        mpkgs = pkgs.pkgsMusl;

        llama-cpp = mpkgs.stdenv.mkDerivation rec {
          pname = "llama-cpp";
          version = "local";
          src = ./deps/llama.cpp;
        
          nativeBuildInputs = [
            mpkgs.cmake
            mpkgs.ninja
            mpkgs.pkgconf
          ];
        
          buildInputs = [ mpkgs.zlib mpkgs.gcc ];
        
          configurePhase = ''
            export CC=${mpkgs.gcc}/bin/gcc
            export CXX=${mpkgs.gcc}/bin/g++
            mkdir -p build
            cmake -S . -B build -G Ninja \
              -DCMAKE_BUILD_TYPE=Release \
              -DBUILD_SHARED_LIBS=OFF \
              -DLLAMA_BUILD_EXAMPLES=OFF \
              -DLLAMA_BUILD_TESTS=OFF \
              -DLLAMA_BUILD_SERVER=OFF \
              -DLLAMA_BUILD_LLAMA_CLI=OFF \
              -DLLAMA_CURL=OFF \
              -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
              -DCMAKE_INSTALL_PREFIX=$out
          '';
        
          buildPhase = ''
            cmake --build build --parallel ''${NIX_BUILD_CORES:-1}
          '';
        
          installPhase = ''
            mkdir -p "$out/lib" "$out/include"
        
            # copy static libs
            find build -type f -name '*.a' -exec cp -v '{}' "$out/lib/" \;
        
            # copy headers
            cp -r include/* "$out/include/" 2>/dev/null || true
            cp -r build/include/* "$out/include/" 2>/dev/null || true
        
            # install pkg-config file if it exists
            if [ -f build/llama.pc ]; then
              mkdir -p "$out/lib/pkgconfig"
              cp build/llama.pc "$out/lib/pkgconfig/"
            fi
          '';
        };

        version = "0.3.6";

      in {
        packages.default = mpkgs.buildGoModule rec {
          pname   = "zeta";
          version = version;
          src     = ./.;

          buildInputs = [ mpkgs.go mpkgs.gcc ];
          nativeBuildInputs = [ mpkgs.musl ];

          # Use the prebuilt binding store path for both library and include.
          env = {
            CGO_ENABLED = "1";
            CC = "${mpkgs.stdenv.cc}";
          };

          # static link against musl
          ldflags = [
            "-s" "-w"
            "-linkmode external"
            "-extldflags '-static -L${mpkgs.musl}/lib -lbinding -lstdc++'"
            "-X main.Version=v${version}"
          ];

          vendorHash = "sha256-RfGPeZ3Tug2CsSV2zkzy+9k3v3KDFsCWvJj8hdVj0SY=";
          doCheck    = false;
        };

        packages.llama = llama-cpp;

        packages.vendor = pkgs.writeShellScriptBin "vendor" ''
          echo "Populating deps directory..."
          rm -rf deps
          mkdir -p deps
          cp -r --no-preserve=mode,ownership ${pkgs.tree-sitter-typst} deps/tree-sitter-typst
          cp -r --no-preserve=mode,ownership ${pkgs.force-graph} deps/force-graph.js
          cp -r --no-preserve=mode,ownership ${pkgs.llama-cpp} deps/llama.cpp

          echo "deps directory is now up to date."
        '';

        devShells.default = pkgs.mkShell {
          shellHook = ''
            cat << "EOF"
            == Welcome to zeta v${version} ==
            - update deps with `nix run .#vendor`
            - test with  `debug`
            EOF
          '';

          buildInputs = let 
            _neovim = (pkgs.neovim.override {
              configure = {
                packages.myPlugins = with pkgs.vimPlugins; {
                  start = [
                    fidget-nvim
                    nvim-notify
                  ];
                };
              };
            });
          in with pkgs; [
            go
            gopls
            gofumpt
            gotools
            golines
            # helper commands that live in the original flake
            (writeShellScriptBin "debug" ''
              rm -rf /tmp/zeta-testing/*
              mkdir -p /tmp/zeta-test-notes
              mkdir -p /tmp/zeta-testing

              go build -o /tmp/zeta-testing/zeta -gcflags=all=-N . || exit

              PATH="/tmp/zeta-testing:$PATH"
              exec ${_neovim}/bin/nvim -u ${./_example/init.lua} /tmp/zeta-test-notes/test.typ
            '')
          ];

          nativeBuildInputs = with pkgs; [ neovim ];
        };
      }
    );
}
