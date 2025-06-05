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
Download the latest release or build from source with
```bash
git clone git@github.com:lentilus/zeta.git
cd zeta && nix build .#zeta
```
> Make sure to place the binary is executable and in neovims runtime path!

## Configuration
Take a look at `_example/init.lua` for how to setup zeta.

## Planned
- [ ] more documentation for usage and setup
- [ ] `zeta --dump` dumps the note-graph as yaml
- [ ] prettier graph view
