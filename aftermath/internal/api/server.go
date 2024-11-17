package api

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"sync"
)

// ConnectionManager manages active connections
type ConnectionManager struct {
	activeConnections int
	mu                sync.Mutex
}

// NewConnection increments the active connection count
func (cm *ConnectionManager) NewConnection() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.activeConnections++
	log.Printf("Active connections: %d", cm.activeConnections)
}

// CloseConnection decrements the active connection count
// and logs if all clients have disconnected
func (cm *ConnectionManager) CloseConnection() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.activeConnections--
	if cm.activeConnections == 0 {
		log.Println("All clients have disconnected.")
	} else {
		log.Printf("Active connections: %d", cm.activeConnections)
	}
}

// JSONRPCServer represents a JSON RPC server
type JSONRPCServer struct {
	handler     any
	handlerName string
	port        int
	connManager *ConnectionManager
}

// NewJSONRPCServer initializes a new JSONRPCServer instance
func NewJSONRPCServer(handler any, handlerName string, port int) *JSONRPCServer {
	return &JSONRPCServer{
		handler:     handler,
		handlerName: handlerName,
		port:        port,
		connManager: &ConnectionManager{},
	}
}

// Start launches the JSON RPC server
func (server *JSONRPCServer) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", server.port))
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	defer listener.Close()

	log.Printf("JSON RPC server is listening on port %d", server.port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		log.Println("New connection!")
		server.connManager.NewConnection()

		go server.handleConnection(conn)
	}
}

// handleConnection handles individual client connections
func (server *JSONRPCServer) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		server.connManager.CloseConnection()
	}()

	rpcServer := rpc.NewServer()
	if err := rpcServer.RegisterName(server.handlerName, server.handler); err != nil {
		log.Printf("Error registering handler %s: %v", server.handlerName, err)
		return
	}

	rpcServer.ServeCodec(jsonrpc.NewServerCodec(conn))
}
