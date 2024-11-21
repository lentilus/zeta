local state = require("aftermath.state")

local M = {}

-- Show error messages
M.error = function(msg)
	if vim.in_fast_event() then
		vim.schedule(function()
			vim.notify("[AM] " .. msg, vim.log.levels.ERROR)
		end)
	else
		vim.notify("[AM] " .. msg, vim.log.levels.ERROR)
	end
end

M.info = function(msg)
	if vim.in_fast_event() then
		vim.schedule(function()
			vim.notify("[AM] " .. msg, vim.log.levels.INFO)
		end)
	else
		vim.notify("[AM] " .. msg, vim.log.levels.INFO)
	end
end

-- Normalize paths by removing trailing slashes
local function normalize(path)
	return path:gsub("/$", "")
end

-- Expand a path to its absolute path
local function expand_path(path)
	local expanded = vim.loop.fs_realpath(path)
	if not expanded then
		M.error(string.format("Invalid path: %s", path))
	end
	return expanded
end

-- Get the relative path of a file within a directory, or nil if outside
local function get_relative_path(directory, file_path)
	directory = normalize(expand_path(directory))
	file_path = normalize(expand_path(file_path))

	if file_path:sub(1, #directory) == directory then
		return file_path:sub(#directory + 2)
	else
		return nil -- File is outside the directory
	end
end

-- Check if a file belongs to the active Zettelkasten
M.is_zettel = function(filepath)
	local root = state.get_path()
	local relative_path = get_relative_path(root, filepath)
	return relative_path ~= nil
end

M.path2zettel = function(filepath)
	local relative = get_relative_path(state.get_path(), filepath)
	return relative
end

return M
