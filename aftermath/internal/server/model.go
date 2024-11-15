package server

import "time"

// Message represents a TCP message received by the server.
type Message struct {
	Type      string    `json:"type"`      // Type of message: request, response, etc.
	ID        string    `json:"id"`        // Unique identifier for the message
	Payload   any       `json:"payload"`   // Specific payload type for incoming messages
	Timestamp time.Time `json:"timestamp"` // Timestamp of the message
}

// RequestPayload represents the structure of an incoming message payload.
type RequestPayload struct {
	Action string   `json:"action"` // Action to be performed
	Args   []string `json:"data"`   // Arguments for the action
}

// ResponsePayload represents the structure of a response to a request.
type ResponsePayload struct {
	Data  []string `json:"data"`
	Error string   `json:"error"`
}
