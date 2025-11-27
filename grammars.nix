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
    {
      name = "mdlink";
      src = pkgs.fetchFromGitHub {
        owner = "lentilus";
        repo = "tree-sitter-mdlink";
        rev = "04acbad7ee3cebb9f612380d0d3fd16f770ee5f1";
        sha256 = "sha256-Xq/36qiOIYmYvSgK230jef4gmIN9p/3wKzb94xP1sW4=";
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
