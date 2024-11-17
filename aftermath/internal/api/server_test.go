package api_test

import (
	"aftermath/internal/api"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"testing"
	"time"
)

// TestExampleMethod tests the ExampleMethod of the Api struct
func TestExampleMethod(t *testing.T) {
	// Start the server in a separate goroutine
	go func() {
		api.StartServer()
	}()

	// Allow the server to start
	time.Sleep(1 * time.Second)

	// Connect to the server as a client
	conn, err := net.Dial("tcp", "127.0.0.1:1234")
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer conn.Close()

	// Create a JSON-RPC client
	client := rpc.NewClientWithCodec(jsonrpc.NewClientCodec(conn))

	// Prepare request and response structures
	params := api.ExampleParams{Name: "John"}
	var result api.ExampleResult

	// Make the RPC call
	err = client.Call("Api.ExampleMethod", &params, &result)
	if err != nil {
		t.Fatalf("RPC call failed: %v", err)
	}

	// Validate the result
	expectedMessage := "Hello, John"
	if result.Message != expectedMessage {
		t.Errorf("Unexpected response: got %q, want %q", result.Message, expectedMessage)
	}
}
