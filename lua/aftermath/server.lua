local utils = require("aftermath.utils")
local state = require("aftermath.state")

local M = {}

-- Function to run an external program with arguments
-- args_table: A table containing the program name as the first argument and other arguments as subsequent elements
local run_external = function(args_table)
	if type(args_table) ~= "table" or #args_table == 0 then
		vim.notify("Invalid arguments: provide a table with program and arguments.", vim.log.levels.ERROR)
		return
	end

	vim.system(args_table, { detach = true })
end

M.start = function()
	local cmd = {
		"/home/lentilus/git/aftermath.nvim.git/dev/bin/aftermath",
		"--port",
		state.get_port(),
		"--root",
		state.get_path(),
		"--cache",
		state.get_path() .. "/.state/index.sqlite",
		"--bib",
		state.get_path() .. "/.state/index.yaml",
	}
	-- Call the helper function with the program and flags
	run_external(cmd)
	utils.info("Started Server.")
end

return M
