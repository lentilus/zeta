package api_test

import (
	"aftermath/internal/api"
	"net"
	"net/rpc/jsonrpc"
	"sync"
	"testing"
	"time"
)

// ExampleHandler is a mock handler used for testing
type ExampleHandler struct{}

// SayHello is a mock method
func (h *ExampleHandler) SayHello(args *string, reply *string) error {
	*reply = "Hello, " + *args
	return nil
}

func TestJSONRPCServer(t *testing.T) {
	// Initialize the server
	handler := &ExampleHandler{}
	server := api.NewJSONRPCServer(handler, "ExampleHandler", 1234)

	// Run the server in a separate goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.Start(); err != nil {
			t.Fatalf("Server failed to start: %v", err)
		}
	}()
	// Give the server some time to start
	time.Sleep(100 * time.Millisecond)

	// Start the client to test the server
	client, err := net.Dial("tcp", "localhost:1234")
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}

	// Create a JSON-RPC client
	rpcClient := jsonrpc.NewClient(client)

	// Prepare the request and response
	args := "World"
	var reply string

	// Call the SayHello method on the server
	err = rpcClient.Call("ExampleHandler.SayHello", &args, &reply)
	if err != nil {
		t.Fatalf("RPC call failed: %v", err)
	}

	// Validate the response
	expected := "Hello, World"
	if reply != expected {
		t.Errorf("Expected reply %q, got %q", expected, reply)
	}
	client.Close()
	wg.Wait()
}
