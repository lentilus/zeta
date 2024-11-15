package server_test

import (
	"aftermath/internal/server"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"
)

func startServer(t *testing.T) {
	// Start the server in a goroutine
	go func() {
		server.StartServer()
	}()
	time.Sleep(1 * time.Second) // Give the server a moment to start
}

func connectClient(id int) {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Printf("Client %d: Error connecting to server: %s\n", id, err)
		return
	}
	defer conn.Close()

	fmt.Printf("Client %d: Connected to server\n", id)

	// Construct a sample request message
	reqPayload := server.RequestPayload{
		Action: "ping",
		Args:   []string{},
	}

	msg := server.Message{
		Type:      "request",
		ID:        fmt.Sprintf("client-%d", id),
		Payload:   reqPayload,
		Timestamp: time.Now(),
	}

	// Marshal the request message to JSON
	reqData, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Client %d: Error marshaling request: %s\n", id, err)
		return
	}

	// Send the request to the server
	conn.Write(append(reqData, '\n')) // Append newline to mark end of the request

	// Read the response from the server
	respBuf := make([]byte, 1024)
	n, err := conn.Read(respBuf)
	if err != nil {
		fmt.Printf("Client %d: Error reading response: %s\n", id, err)
		return
	}

	// Parse the response
	var resp server.Message
	err = json.Unmarshal(respBuf[:n], &resp)
	if err != nil {
		fmt.Printf("Client %d: Error unmarshaling response: %s\n", id, err)
		return
	}

	// Output the response
	fmt.Printf("Client %d: Received response: %+v\n", id, resp)
}

func TestServer(t *testing.T) {
	startServer(t)

	go connectClient(1)
	go connectClient(2)
	go connectClient(3)

	// Ugly fix to avoid race condition
	time.Sleep(10 * time.Second)
}
