package graph

import (
	"embed"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// GraphData holds the nodes and links of the graph.
type GraphData struct {
	Nodes []Node `json:"nodes"`
	Links []Link `json:"links"`
}

// Node represents a graph node.
// ID must be unique.
type Node struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// Link represents a directed edge between two nodes.
type Link struct {
	Source int `json:"source"`
	Target int `json:"target"`
}

// IncrementalMessage is sent over WebSocket to update clients.
type IncrementalMessage struct {
	Op    string     `json:"op"`              // "init", "add", "update", "deleteNode", "deleteLink"
	Graph *GraphData `json:"graph,omitempty"` // used for "init"
	Node  *Node      `json:"node,omitempty"`  // for add/update/deleteNode
	Link  *Link      `json:"link,omitempty"`  // for add/deleteLink
}

//go:embed static/*
var staticFiles embed.FS

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

var (
	graph   = GraphData{Nodes: []Node{}, Links: []Link{}}
	graphMu sync.Mutex

	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex
)

// ShowGraph starts the HTTP and WebSocket server on the given address (e.g. ":8080").
// It returns the URI where the graph can be viewed (e.g. "http://localhost:8080/").
func ShowGraph(addr string) string {
	// Listen on the given address (":0" means any free port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Could not start listener: %v", err)
	}

	// Extract the actual address (useful if addr was ":0")
	actualAddr := l.Addr().String()

	// Setup handlers
	http.Handle("/", http.FileServer(http.FS(staticFiles)))
	http.HandleFunc("/ws", handleWS)

	// Start serving
	go func() {
		if err := http.Serve(l, nil); err != nil {
			log.Printf("Graph server error: %v", err)
		}
	}()

	// Return full URL to UI
	return "http://" + actualAddr + "/static/"
}

// AddNode adds a node to the graph and broadcasts the change.
func AddNode(node Node) error {
	graphMu.Lock()
	graph.Nodes = append(graph.Nodes, node)
	graphMu.Unlock()
	msg := IncrementalMessage{Op: "add", Node: &node}
	return broadcastMessage(msg)
}

// UpdateNode updates an existing node (matched by ID) and broadcasts.
func UpdateNode(node Node) error {
	graphMu.Lock()
	for i, n := range graph.Nodes {
		if n.ID == node.ID {
			graph.Nodes[i] = node
			break
		}
	}
	graphMu.Unlock()
	msg := IncrementalMessage{Op: "update", Node: &node}
	return broadcastMessage(msg)
}

// DeleteNode removes a node by ID and broadcasts.
func DeleteNode(nodeID int) error {
	graphMu.Lock()
	// remove node
	newNodes := make([]Node, 0, len(graph.Nodes))
	for _, n := range graph.Nodes {
		if n.ID != nodeID {
			newNodes = append(newNodes, n)
		}
	}
	graph.Nodes = newNodes
	graphMu.Unlock()
	msg := IncrementalMessage{Op: "deleteNode", Node: &Node{ID: nodeID}}
	return broadcastMessage(msg)
}

// AddLink adds a link to the graph and broadcasts.
func AddLink(link Link) error {
	graphMu.Lock()
	graph.Links = append(graph.Links, link)
	graphMu.Unlock()
	msg := IncrementalMessage{Op: "add", Link: &link}
	return broadcastMessage(msg)
}

// DeleteLink removes a link (exact match) and broadcasts.
func DeleteLink(link Link) error {
	graphMu.Lock()
	newLinks := make([]Link, 0, len(graph.Links))
	for _, l := range graph.Links {
		if !(l.Source == link.Source && l.Target == link.Target) {
			newLinks = append(newLinks, l)
		} else {
			log.Println("DELETING a link")
		}
	}
	graph.Links = newLinks
	graphMu.Unlock()
	msg := IncrementalMessage{Op: "deleteLink", Link: &link}
	return broadcastMessage(msg)
}

// GetGraph returns a snapshot of the current graph.
func GetGraph() GraphData {
	graphMu.Lock()
	defer graphMu.Unlock()
	// shallow copy sufficient for read-only
	copy := GraphData{
		Nodes: append([]Node{}, graph.Nodes...),
		Links: append([]Link{}, graph.Links...),
	}
	return copy
}

// broadcastMessage marshals and sends a message to all clients.
func broadcastMessage(msg IncrementalMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for conn := range clients {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Broadcast error: %v", err)
			conn.Close()
			delete(clients, conn)
		}
	}
	return nil
}

// handleWS upgrades HTTP connections and sends initial graph state.
func handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS upgrade error: %v", err)
		return
	}
	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()
	defer func() {
		clientsMu.Lock()
		delete(clients, conn)
		clientsMu.Unlock()
		conn.Close()
	}()

	// send initial graph
	state := GetGraph()
	initMsg := IncrementalMessage{Op: "init", Graph: &state}
	if data, err := json.Marshal(initMsg); err == nil {
		_ = conn.WriteMessage(websocket.TextMessage, data)
	} else {
		log.Printf("Init marshal error: %v", err)
	}

	// keep connection open
	for {
		if _, _, err := conn.NextReader(); err != nil {
			break
		}
	}
}
