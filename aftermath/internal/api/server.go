package api

import (
	"context"
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
	onZeroConns       func()
}

// NewConnectionManager creates a new ConnectionManager
func NewConnectionManager(onZeroConns func()) *ConnectionManager {
	return &ConnectionManager{
		onZeroConns: onZeroConns,
	}
}

// NewConnection increments the active connection count
func (cm *ConnectionManager) NewConnection() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.activeConnections++
	log.Printf("Active connections: %d", cm.activeConnections)
}

// CloseConnection decrements the active connection count
// and triggers server shutdown if all clients have disconnected
func (cm *ConnectionManager) CloseConnection() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.activeConnections--
	if cm.activeConnections == 0 {
		log.Println("All clients have disconnected.")
		if cm.onZeroConns != nil {
			go cm.onZeroConns()
		}
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
	listener    net.Listener
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{}
}

// NewJSONRPCServer initializes a new JSONRPCServer instance
func NewJSONRPCServer(handler any, handlerName string, port int) *JSONRPCServer {
	ctx, cancel := context.WithCancel(context.Background())
	server := &JSONRPCServer{
		handler:     handler,
		handlerName: handlerName,
		port:        port,
		ctx:         ctx,
		cancel:      cancel,
		done:        make(chan struct{}),
	}

	server.connManager = NewConnectionManager(server.shutdown)
	return server
}

// shutdown closes the server
func (server *JSONRPCServer) shutdown() {
	log.Println("Shutting down server...")
	server.cancel()
	if server.listener != nil {
		server.listener.Close()
	}
	close(server.done)
}

// Start launches the JSON RPC server
func (server *JSONRPCServer) Start() error {
	var err error
	server.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", server.port))
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}
	defer server.listener.Close()

	log.Printf("JSON RPC server is listening on port %d", server.port)

	for {
		select {
		case <-server.ctx.Done():
			log.Println("Server shutdown initiated")
			return nil
		default:
			conn, err := server.listener.Accept()
			if err != nil {
				if server.ctx.Err() != nil {
					// Server is shutting down
					return nil
				}
				log.Printf("Error accepting connection: %v", err)
				continue
			}

			log.Println("New connection!")
			server.connManager.NewConnection()

			go server.handleConnection(conn)
		}
	}
}

// Wait blocks until the server is shut down
func (server *JSONRPCServer) Wait() {
	<-server.done
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
