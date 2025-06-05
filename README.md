# zeta $\zeta$

![GitHub Tag](https://img.shields.io/github/v/tag/lentilus/zeta?label=version)
![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/lentilus/zeta/release.yaml)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/lentilus/zeta/total)

A typst language-server for __zettelkasten__-style note-taking with reference tracking and navigation.

<p style="display: flex; justify-content: space-between; margin: 0;">
  <img src="https://github.com/user-attachments/assets/728379af-0e0d-49b4-82cf-aa19fb65cbe0" width="32%" />
  <img src="https://github.com/user-attachments/assets/1c5e9ef4-48d1-45e1-bed5-ef119c1465c9" width="32%" />
  <img src="./_example/zeta-demo14.gif" width="32%" />
</p>


## Language Server Features
1. **Go to Definition** navigates directly to referenced notes.
2. **Find References** locates all notes that reference the current note (backlinks).
3. **Workspace Symbols** show all notes by name and path. __(Best used with Telescope)__
4. **Document Diagnostics** hint a links resolved path.

## Installation
Download the latest [release](https://github.com/lentilus/zeta/releases/latest). Make the binary executable and place it in your path. _Done!_

<details>

<summary>Build from source (with nix) </summary>

### on any host
Clone the repo and build the binary with nix.
```bash
git clone git@github.com:lentilus/zeta.git
cd zeta && nix build .#zeta
```
_The binary is statically linked. Nix is only needed for the build._

### in a nix flake
```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    zeta = {
      url = "github:lentilus/zeta";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    ...
  };

  outputs = { nixpkgs, zeta, ... }: let
    system = "x86_64-linux";
    pkgs = import nixpkgs { inherit system; };
    zeta = zeta.packages.${system}.zeta;
  in {
    ...
  };
}
``` 

</details>

## Configuration
Zeta is configured entirely through the `initialization_options`. Below is an example for neovim. The setup for other editors is analogous.
```lua
vim.lsp.config['zeta'] = {
  cmd = { 'zeta' },
  filetypes = { 'typst' },
  root_markers = { 'test.typ' },
  init_options = {} -- zeta has defaults
  on_attach = function()
      print("Zeta attached!")
  end,

}
vim.lsp.enable('zeta')
```
The default `init_options` are
```lua
defaults = {
  -- A treesitter query that is used to extract the necessary
  -- information from notes. The @target capture is mandatory.
  -- It is piped through the select_regex to extract a references target.
  query = [[
    (code (call item: (ident) @link (#eq? @link "link") (group (string) @target )))
    (heading (text) @title) 
    (heading (label) @taxon) 
  ]],

  -- These are the names of the captures that are used to generate a notes title
  -- The captured values are plugged into the title template in the order they appear.
  title_substitutions = {"taxon", "title"},
  title_template = "%s %s",

  -- The regex used to select a substring from the tree-sitter @target capture.
  -- If the regex yields multiple captures, the first is used.
  select_regex = '^"(.*)"$',

  -- The default extension to use if a target does not have one.
  default_extension = ".typ",

  -- The file extension's of files that zeta should look at.
  -- This is especially important for zeta to be able to detect notes not opened in
  -- the editor.
  file_extensions = {".typ"},
}
```
## Contribute
Contributions are very welcome!
