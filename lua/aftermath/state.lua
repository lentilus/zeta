local M = {}

local state = {
	path = nil,
	port = nil,
}

M.setup = function(path, port)
	state.path = path or "/home/lentilus/typstest"
	state.port = port or 1234
end

M.switch = function(path, port)
	state.path = path
	state.port = port
end

M.get_path = function()
	return state.path
end

M.get_port = function()
	return state.port
end

return M
