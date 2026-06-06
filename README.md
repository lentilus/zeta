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

  -- While writing ordinary text, suggest turning the phrase you are typing
  -- into a link when it fuzzily matches an existing note's title (the whole
  -- phrase is replaced with #link("id")[Title]). Set false to disable.
  suggest_links_in_text = true,

  -- Minimum fuzzy closeness (0..1) for a prose suggestion to appear. A clean
  -- (even partial) prefix scores 1.0; typos lower it. Raise to get fewer
  -- suggestions, lower for looser matching.
  link_suggest_threshold = 0.7,

  -- Link-target completion lets you search notes by title and inserts the
  -- (arbitrary) filename for you, so you link concepts, not filenames.
  -- When true, accepting a completion also fills an empty/absent display
  -- body, e.g. #link("d654bf")[Axiom of Choice]. Existing bodies are kept.
  -- Set to false if your links/query do not use a display body.
  -- On clients that support snippets the inserted body is selected so you
  -- can immediately keep or overwrite it.
  completion_insert_display = true,

  -- The text inserted into the display body is built separately from the
  -- title, so classifiers used only to disambiguate notes in the picker
  -- (e.g. a <Tool>/<Theorem> taxon) are kept out of the rendered link.
  -- Defaults below insert just the title capture.
  display_substitutions = {"title"},
  display_template = "%s",

  -- How the filename is generated when you create a note from a link:
  --   "random"    -> opaque short id, e.g. a1b2c3.typ (default)
  --   "timestamp" -> e.g. 20260606T153012.typ
  --   "slug"      -> derived from the title, e.g. axiom-of-choice.typ
  new_note_id_scheme = "random",

  -- Content written to a newly created note; %s is the title.
  new_note_template = "= %s\n",

  -- Optional external command used to create notes instead of the template
  -- (lets an external system enforce its own formatting/naming). The
  -- placeholders {title}, {path}, {id} and {root} are substituted. Empty
  -- means use new_note_template.
  new_note_command = "",
}
```

> Note creation (the "Create note" completion item and the `createNote`
> code action) materializes the file on clients that run completion-item
> commands (e.g. Neovim) or via the code action (e.g. Helix). Otherwise the
> file is created the first time you navigate to the link and save.
## Contribute
Contributions are very welcome!
