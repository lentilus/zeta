{pkgs, ...}: let
  grammars = [
    {
      name = "typst";
      src = pkgs.fetchFromGitHub {
        owner = "uben0";
        repo = "tree-sitter-typst";
        rev = "46cf4ded12ee974a70bf8457263b67ad7ee0379d";
        sha256 = "sha256-s/9R3DKA6dix6BkU4mGXaVggE4bnzOyu20T1wuqHQxk=";
      };
    }
  ];

  buildGrammar = {
    name,
    src,
  }:
    pkgs.stdenv.mkDerivation {
      pname = "tree-sitter-${name}";
      version = "v0.0.1";
      inherit src;

      buildPhase = "echo 'No build required for source files'";

      installPhase = ''
        runHook preInstall
        mkdir -p $out/${name}
        cp -r ./src/* $out/${name}
        runHook postInstall
      '';
    };
in
  pkgs.symlinkJoin {
    name = "tree-sitter-all-grammars";
    paths = map buildGrammar grammars;
  }
