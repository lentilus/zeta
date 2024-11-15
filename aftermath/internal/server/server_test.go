package server_test

import (
	"aftermath/internal/server"
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

	// Simulate some interaction with the server
	time.Sleep(3 * time.Second)

	// The client disconnects
	fmt.Printf("Client %d: Disconnected\n", id)
}

func TestServer(t *testing.T) {
	startServer(t)

	// Client 1 connects
	go connectClient(1)

	// Client 2 connects after 2 seconds
	time.Sleep(1 * time.Second)
	go connectClient(2)

	// Client 3 connects after 4 seconds
	time.Sleep(1 * time.Second)
	go connectClient(3)

	// Ugly fix to avoid race condition
	time.Sleep(10 * time.Second)
}
