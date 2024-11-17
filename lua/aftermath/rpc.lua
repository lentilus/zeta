local M = {}

-- Store the connection details
local client = {
	host = nil,
	port = nil,
	socket = nil,
	id = 0,
	pending_requests = {},
}

-- Initialize connection parameters
function M.setup(host, port)
	client.host = host or "localhost"
	client.port = port or 1234
end

-- Connect to the server
local function connect()
	if client.socket then
		return
	end

	local socket = vim.loop.new_tcp()
	local connect_success = vim.loop.tcp_connect(socket, client.host, client.port, function() end)

	if not connect_success then
		error(string.format("Failed to connect to %s:%d", client.host, client.port))
	end

	client.socket = socket
end

-- Close the connection
function M.close()
	if client.socket then
		client.socket:close()
		client.socket = nil
	end
end

-- Handle incoming responses
local function handle_response(response)
	local decoded = vim.json.decode(response)
	if decoded.id then
		local callback = client.pending_requests[decoded.id]
		if callback then
			callback(decoded.result, decoded.error)
			client.pending_requests[decoded.id] = nil
		end
	end
end

-- Read from the socket
local function start_read()
	local buffer = ""

	client.socket:read_start(function(err, chunk)
		if err then
			error("Read error: " .. err)
		end

		if chunk then
			buffer = buffer .. chunk

			-- Try to find complete JSON-RPC messages
			local start, end_pos = buffer:find("\n")
			while start do
				local message = buffer:sub(1, end_pos - 1)
				buffer = buffer:sub(end_pos + 1)
				handle_response(message)
				start, end_pos = buffer:find("\n")
			end
		end
	end)
end

-- Send a request and wait for response
function M.request(method, params)
	if not client.socket then
		connect()
		start_read()
	end

	client.id = client.id + 1
	local current_id = client.id

	-- Format the method name to match Go's expectations
	local full_method = "Api." .. method

	local request = {
		method = full_method,
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
		error("Failed to send request")
	end

	-- Wait for response using vim.wait()
	vim.wait(5000, function()
		return response ~= nil or error_response ~= nil
	end)

	if error_response ~= vim.NIL then
		error(string.format("RPC error: %s", vim.inspect(error_response)))
	end

	return response
end

-- Testing
local rpc = M

rpc.setup("127.0.0.1", 1234)

local result1 = rpc.request("ExampleMethod", { name = "John" })
local result2 = rpc.request("ExampleMethod", { name = "Joe" })
local result3 = rpc.request("ExampleMethod", { name = "Foo" })
print(vim.inspect(result1))
print(vim.inspect(result2))
print(vim.inspect(result3))
