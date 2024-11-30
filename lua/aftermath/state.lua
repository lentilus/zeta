local M = {}

local state = {
	path = nil,
	port = nil,
}

-- FNV-1a hash function to generate a unique port number from a file path
local hash_filepath_to_port = function(filepath)
	local fnv_prime = 16777619
	local hash = 2166136261 -- FNV offset basis

	for i = 1, #filepath do
		local byte = filepath:byte(i)
		hash = hash ~ byte
		hash = (hash * fnv_prime) % 2 ^ 32
	end

	-- Convert the 32-bit hash into a valid port number (1024-65535)
	local port = 1024 + (hash % (65535 - 1024))

	return port
end

M.setup = function(path)
	state.path = path or "/home/lentilus/typstest"
	state.port = hash_filepath_to_port(path)
end

M.switch = function(path)
	state.path = path
	state.port = hash_filepath_to_port(path)
end

M.get_path = function()
	return state.path
end

M.get_port = function()
	return state.port
end

return M
