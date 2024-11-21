local rpc = require("aftermath.rpc")
local state = require("aftermath.state")
local utils = require("aftermath.utils")

local M = {}

local function request(func, args)
	local res = rpc.request("API." .. func, args)
	if not res then
		error("No response")
	end
	if res.error and res.error ~= "<nil>" then
		error(res.error)
	end
	return res.zettels
end

M.setup = function()
	rpc.setup("127.0.0.1", state.get_port())
	rpc.connect()
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

M.getall = function()
	return request("GetAll", { zettel = "foo" })
end

return M
