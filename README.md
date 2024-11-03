# The below is just an OUTLINE
If you found this project and want to contribute: Feel free to do so. Below is a general outline for how I imagine the final plugin.

---

# aftermath

> A Neovim Zettelkasten (slip box) plugin facilitating very fast indexing of Typst-based slip boxes.

## Layout
A slip box is a directory containing Typst files.  
A shared `index.bib` holds all objects that can be referenced.  
Why use a `.bib` file? Bib files prove to be a versatile way of specifying references to different sorts of resources. Typst provides first-class support for using bib files.  
Zettels and other resources can simply be referenced by adding `@linkedzettel` in the source code.  
Leveraging the `@` syntax allows for much faster indexing using tree-sitter queries, compared to using a more Typst-esque approach that would require slower Typst queries.

## Indexing
*aftermath* caches the relationship of files in an SQLite database. The database has an entry for every zettel file, along with a checksum and forward and backward links. The checksums allow incremental updates to the index cache.  
The links from a zettel are retrieved via a tree-sitter query for any references in the document.

## Stack
- Neovim plugin with Lua API
- Core written in Go
- Tree-sitter Typst bindings integrated with cgo
