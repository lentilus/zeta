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
  -- the editor
  file_extensions = {".typ"},
}

local on_attach = function(client, bufnr)
  print("LSP attached to buffer", bufnr)
  
  local function buf_set_keymap(...) vim.api.nvim_buf_set_keymap(bufnr, ...) end
  local opts = { noremap=true, silent=true }
  
  buf_set_keymap('n', 'gd', '<cmd>lua vim.lsp.buf.definition()<CR>', opts)
  buf_set_keymap('n', 'gr', '<cmd>lua vim.lsp.buf.references()<CR>', opts)
  
  vim.api.nvim_buf_create_user_command(bufnr, "ZetaGraph", function()
    client.request(
      "workspace/executeCommand",
      { command = "graph", arguments = {} },
      function(err, result)
        if err then
          vim.notify("Error executing graph command: " .. err.message, vim.log.levels.ERROR)
        else
          vim.notify("Graph command executed.")
        end
      end,
      bufnr
    )
  end, { desc = "Execute Zeta LSP 'graph' command" })
end

vim.lsp.config['zeta'] = {
  cmd = { 'zeta', '--logfile=/tmp/zeta.log' },
  filetypes = { 'typst' },
  init_options = defaults,
  root_markers = { 'test.typ' },
  on_attach = function()
      print("Zeta attached!")
  end,

}

vim.lsp.enable('zeta')
