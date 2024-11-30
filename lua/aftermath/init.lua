local api = require("aftermath.api")
local state = require("aftermath.state")
local utils = require("aftermath.utils")

local M = {}

local function register_hooks()
	vim.api.nvim_create_autocmd("BufWritePost", {
		group = vim.api.nvim_create_augroup("AftermathHooks", { clear = true }),
		callback = function(event)
			local filepath = event.file
			if not utils.is_zettel(filepath) then
				return
			end
			local ok, err = pcall(api.update, filepath)
			if not ok then
				utils.error("Aftermath Update Error: " .. err)
			end
			utils.info("Updated Links.")
		end,
	})
end

M.setup = function(initial_path, initial_port)
	initial_path = initial_path or "/home/lentilus/typstest"
	initial_port = initial_port or 1234
	state.setup(initial_path, initial_port)
	api.setup()
	register_hooks()
end

M.switch_zettelkasten = function(new_path, new_port)
	state.switch(new_path, new_port)
	api.switch()
	register_hooks() -- Clear old hooks and register new ones
end

return M
