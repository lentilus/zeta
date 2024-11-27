local utils = require("aftermath.utils")
local server = require("aftermath.server")

local M = {}

local client = {
	host = nil,
	port = nil,
	socket = nil,
	id = 0,
	pending_requests = {},
	buffer = "",
}

function M.setup(host, port)
	client.host = host or "localhost"
	client.port = port or 1234
end

-- Handle incoming responses
local function handle_response(response)
	local decoded = vim.json.decode(response)
	if decoded and decoded.id then
		local callback = client.pending_requests[decoded.id]
		if callback then
			callback(decoded.result, decoded.error)
			client.pending_requests[decoded.id] = nil
		end
	end
end

-- Start reading from the socket
local function start_read()
	if not client.socket then
		error("Socket is not initialized")
	end

	client.socket:read_start(function(err, chunk)
		if err then
			vim.notify("RPC read error: " .. err, vim.log.levels.ERROR)
			M.close()
			return
		end

		if chunk then
			client.buffer = client.buffer .. chunk

			-- Extract complete JSON-RPC messages (terminated by newline)
			local start, finish = client.buffer:find("\n")
			while start do
				local message = client.buffer:sub(1, finish - 1)
				client.buffer = client.buffer:sub(finish + 1)
				handle_response(message)
				start, finish = client.buffer:find("\n")
			end
		end
	end)
end

-- Connect to the server
function M.connect(startup)
	startup = startup or false

	-- if client.socket then
	-- 	return
	-- end

	local socket = vim.loop.new_tcp()
	socket:connect(client.host, client.port, function(err)
		if err then
			if startup then
				-- start backend server
				server.start()
				-- connect to it
				vim.defer_fn(M.connect, 20)
			else
				local msg = string.format("Failed to connect to %s:%d: %s", client.host, client.port, err)
				utils.error(msg)
			end
		else
			utils.info("Server connected.")
		end

		client.socket = socket
		start_read()
	end)
end

-- Close the connection
function M.close()
	if client.socket then
		client.socket:read_stop()
		client.socket:close()
		client.socket = nil
		client.buffer = ""
		client.pending_requests = {}
	end
end

-- Reconnect to the server
function M.reconnect(host, port)
	M.close()
	M.setup(host, port)
	M.connect()
end

-- Send a request and wait for response
function M.request(method, params)
	if not client.socket then
		M.connect()
	end

	client.id = client.id + 1
	local current_id = client.id

	local request = {
		method = method,
		params = { params }, -- Wrap params in array as Go expects
		id = current_id,
	}

	local response = nil
	local error_response = nil

	-- Create a callback to receive the response
	client.pending_requests[current_id] = function(result, err)
		response = result
		error_response = err
	end

	-- Send the request
	local success = client.socket:write(vim.json.encode(request) .. "\n")
	if not success then
		utils.error("Failed to send request")
	end

	-- Wait for response using vim.wait()
	local timeout = 500
	vim.wait(timeout, function()
		return response ~= nil or error_response ~= nil
	end, 10)

	-- Check for timeout
	if not response and not error_response then
		client.pending_requests[current_id] = nil -- Cleanup
		utils.error(string.format("Request timed out after %dms", timeout))
	end

	if error_response ~= vim.NIL then
		utils.error(string.format("RPC error: %s", vim.inspect(error_response)))
	end

	return response
end

return M
