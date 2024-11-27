local rpc = require("aftermath.rpc")
local state = require("aftermath.state")
local utils = require("aftermath.utils")

local M = {}

local function request(func, args)
	local res = rpc.request("API." .. func, args)
	if not res or res == vim.NIL then
		print(vim.inspect(res))
		error("No response. Invalid endpoint?")
	end
	if res.error and res.error ~= "<nil>" then
		error(res.error)
	end
	return res.zettels
end

M.setup = function()
	rpc.setup("127.0.0.1", state.get_port())
	rpc.connect(true)
end

M.switch = function(path, port)
	state.switch(path, port)
	rpc.reconnect("127.0.0.1", state.get_port())
end

M.update = function(filepath)
	if not utils.is_zettel(filepath) then
		error("File is outside the active Zettelkasten")
	end

	return request("Update", { zettel = vim.loop.fs_realpath(filepath) })
end

M.get_index = function()
	return request("GetAll", { zettel = "dummy" })
end

M.get_children = function(filename)
	return request("GetForwardLinks", { zettel = filename })
end

M.get_parents = function(filename)
	return request("GetBackLinks", { zettel = filename })
end

return M
