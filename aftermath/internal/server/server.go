package server

import (
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func handleClient(
	conn net.Conn,
	stopChan chan struct{},
	mu *sync.Mutex,
	clientCount *int,
	wg *sync.WaitGroup,
) {
	defer conn.Close()

	mu.Lock()
	*clientCount++
	mu.Unlock()

	log.Printf("New client connected: %s", conn.RemoteAddr())

	// Simulate interaction with the client
	buf := make([]byte, 1024)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			// When client closes connection or an error occurs
			log.Printf("Client disconnected: %s", conn.RemoteAddr())
			break
		}
	}

	// Decrease the client count when a client disconnects
	mu.Lock()
	*clientCount--
	mu.Unlock()

	// Signal that this client's work is finished
	wg.Done()

	mu.Lock()
	if *clientCount == 0 {
		mu.Unlock()
		log.Println("All clients disconnected. Stopping the server...")
		stopChan <- struct{}{} // Send a signal to stop the server
		return
	}
	mu.Unlock()
}

func StartServer() {
	// Initialize logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Listen on TCP port 8080
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
		return
	}
	defer ln.Close()

	log.Println("Server started. Waiting for clients...")

	// Setup signal catching to gracefully shutdown the server
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Channel to signal when all clients have disconnected
	stopChan := make(chan struct{})

	// Variables for tracking client count and synchronization
	var mu sync.Mutex
	var clientCount int
	var wg sync.WaitGroup

	for {
		select {
		case <-sigChan: // Shutdown signal
			log.Println("Shutdown signal received, stopping server...")
			return
		case <-stopChan: // All clients disconnected
			return
		default: // New connections
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("Error accepting client: %v", err)
				continue
			}

			wg.Add(1)
			go handleClient(conn, stopChan, &mu, &clientCount, &wg)
		}
	}

	// Wait for all clients to disconnect
	wg.Wait()
	log.Println("Server shutdown completed.")
}
