package server

import (
	"fmt"
	"time"
)

// handleRequest processes a request based on the given Message
func handleRequest(msg Message) Message {
	var response Message
	response.Type = "response"
	response.ID = msg.ID
	response.Timestamp = time.Now()

	// Type assertion for Payload
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		response.Payload = ResponsePayload{
			Data:  nil,
			Error: "Invalid payload format",
		}
		return response
	}

	// Parse the payload into a structured RequestPayload
	var reqPayload RequestPayload
	if action, exists := payload["action"].(string); exists {
		reqPayload.Action = action
	}
	if args, exists := payload["data"].([]interface{}); exists {
		for _, arg := range args {
			if str, ok := arg.(string); ok {
				reqPayload.Args = append(reqPayload.Args, str)
			}
		}
	}

	// Handle the request based on the action
	switch reqPayload.Action {
	case "ping":
		response.Payload = ResponsePayload{
			Data:  []string{"pong"},
			Error: "",
		}
	default:
		response.Payload = ResponsePayload{
			Data:  nil,
			Error: fmt.Sprintf("Unknown action: %s", reqPayload.Action),
		}
	}

	return response
}
