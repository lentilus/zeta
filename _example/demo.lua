local init_options = {
  -- `query` defines the treesitter query used to find links and other metadata.
  -- You can put anything here - be creative!
  -- NOTE: The capture that specifies a link must be named `target`!
  -- Use the `title` capture if the filename is not semantic
  query = [[
    (code (call item: (call item: (ident) @link (#eq? @link "link") (group (string) @target ))))
    (heading (text) @title) 
    (heading (label) @taxon) 
  ]],
  
  -- `select_regex` selects a substring of the `target` capture as the reference.
  select_regex = '^"(.*)"$',
  default_extension = ".typ",
  file_extensions = {".typ"},

}

-- diagnostic messages
vim.diagnostic.config {
  update_in_insert = true,
  virtual_text = {
    source = 'if_many',
    spacing = 4,
  },
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


vim.api.nvim_create_autocmd({ "BufReadPost", "BufNewFile" }, {
  pattern = "*.typ",
  callback = function()
    vim.lsp.start({
      name = "zeta",
      cmd  = { "/tmp/zeta-testing/zeta", "--logfile=/tmp/zeta.log"},
      filetypes = { "typst" },
      root_dir  = "/tmp/zeta-demo-notes",
      capabilities = vim.lsp.protocol.make_client_capabilities(),
      single_file_support = true,
      init_options = init_options,
      on_attach = on_attach,
    })
  end,
})
