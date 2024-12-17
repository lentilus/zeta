# zeta $\zeta$

A highly performant language server for __zettelkasten__-style note-taking in __typst__ that provides real-time reference tracking and navigation.

## Highlights

- **Perfomant**: Uses incremental parsing and concurrent processing for real-time updates
- **Consistant**: Seemless in-background cache validation ensures consitency
- **Configurable**: Uses tree-sitter for robust reference parsing with configurable queries
- **Efficient**: SQLite-based persistent cache for fast startup and reference lookups
- **Integrated**: Maintains a Hayagriva bibliography of all notes for easy integration with typst

## Language Server Features

1. **Go to Definition**: Navigate directly to referenced notes
2. **Find References**: Locate all notes that reference the current note
3. **Document Diagnostics**: Real-time hints on references

## Configuration

The language server can be configured through lsp-config.
