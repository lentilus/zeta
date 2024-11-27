# Warning

This project is in a very early stage and I am still working out the core components. There will be breaking changes in the api until I finalize it. This project is not ready to be used!

--- 

# Aftermath.nvim

**Aftermath.nvim** is a Neovim plugin that provides real-time link management and navigation for Zettelkasten-style Typst projects. The plugin runs seamlessly in the background, detecting and updating `@links` as you work, leveraging [Tree-sitter](https://tree-sitter.github.io/) for precise link identification within your notes. The plugin manages an sqlite cache that is updated incrementally making queries insanely fast.

## Features

- **Automatic Link Updates:** Updates forward and backward links whenever you save a file.
- **Zettelkasten Management:** Switch between different Zettelkasten directories effortlessly.
- **Telescope Integration:** Browse note indices and navigate child and parent links using [Telescope.nvim](https://github.com/nvim-telescope/telescope.nvim).

## Installation

### Prerequisites

- [Neovim](https://neovim.io/) (v0.7+)
- [Tree-sitter](https://tree-sitter.github.io/) for syntax-based link identification
- [Telescope.nvim](https://github.com/nvim-telescope/telescope.nvim) for link browsing (optional but recommended)

## Setup

After installing, set up Aftermath.nvim in your `init.lua`:

```lua
require('aftermath').setup('/path/to/zettelkasten', 1234)
```

### Optional: Telescope Integration

To enable Telescope pickers for navigating notes, register the extension:

```lua
require('telescope').load_extension('aftermath')
```

You can then use:

- `:Telescope aftermath index` – Show all indexed notes.
- `:Telescope aftermath children` – Show notes linked *from* the current note.
- `:Telescope aftermath parents` – Show notes linking *to* the current note.

## Key Features and Usage

### 1. **Automatic Link Updates**
Aftermath.nvim automatically updates links whenever you save a file in your Zettelkasten. No manual link management is required. The plugin monitors file changes and updates forward and backward references automatically.
Every 5 minutes the cache is validated regardless of activity in neovim to ensure its integrety.

### 2. **Switching Zettelkasten Directories**
Switch between multiple Zettelkasten repositories:

```lua
require('aftermath').switch_zettelkasten('/new/zettelkasten/path', 1234)
```

This updates the active Zettelkasten root and reconnects to the backend server.

## Go Backend Server

Aftermath.nvim uses a Go server as the backend, which handles:

- Caching of note links in an SQLite database.
- Incremental updates every 5 minutes.
- A JSON-RPC API for efficient data retrieval.

The server starts automatically when Neovim launches the plugin.
**Note:** The server is designed to run as a background service, and the plugin automatically connects to it.

## Contributing

Contributions are welcome! Feel free to submit issues or pull requests on the [GitHub repository](https://github.com/lentilus/aftermath.nvim).
