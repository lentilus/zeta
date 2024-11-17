package api

import (
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"sync"
)

// Api represents the server handler
type Api struct {
}

// ExampleParams represents parameters for a method
type ExampleParams struct {
	Name string `json:"name"`
}

// ExampleResult represents the result of a method
type ExampleResult struct {
	Message string `json:"message"`
}

// ExampleMethod handles the "ExampleMethod" request
func (a *Api) ExampleMethod(params *ExampleParams, result *ExampleResult) error {
	// Process the request (here we just respond with a success message)
	result.Message = "Hello, " + params.Name
	return nil
}

var activeConnections = 0
var mu sync.Mutex

// StartServer starts the JSON RPC server
func StartServer() {
	listener, err := net.Listen("tcp", ":1234")
	if err != nil {
		log.Fatal("Error starting server:", err)
	}
	defer listener.Close()

	log.Println("JSON RPC server is listening on port 1234...")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting connection:", err)
			continue
		}
		log.Println("New connection!")

		mu.Lock()
		activeConnections++
		mu.Unlock()

		go func(conn net.Conn) {
			defer func() {
				conn.Close()
				mu.Lock()
				activeConnections--
				if activeConnections == 0 {
					log.Println("All clients have disconnected.")
				}
				mu.Unlock()
			}()

			// Create a new rpc.Server and register the handler
			server := rpc.NewServer()
			if err := server.RegisterName("Api", &Api{}); err != nil {
				log.Println("Error registering API:", err)
				return
			}

			// Serve the connection using the JSON-RPC codec
			server.ServeCodec(jsonrpc.NewServerCodec(conn))
		}(conn)
	}
}
