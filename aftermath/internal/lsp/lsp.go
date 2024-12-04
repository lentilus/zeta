package lsp

import (
	"log"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
	"github.com/tliron/glsp/server"
)

const lsName = "aftermath"

var version string = "0.0.1"

type LanguageServer struct {
	state   string
	handler protocol.Handler
}

// NewServer initializes and returns a new LSP server instance.
func NewServer() *server.Server {
	ls := &LanguageServer{}

	// Initialize the protocol handler
	ls.handler = protocol.Handler{
		Initialize:              ls.initialize,
		Initialized:             ls.initialized,
		Shutdown:                ls.shutdown,
		SetTrace:                ls.setTrace,
		WorkspaceExecuteCommand: ls.executeCommand,
	}

	// Create the LSP server
	return server.NewServer(&ls.handler, lsName, false)
}

func (ls *LanguageServer) initialize(
	context *glsp.Context,
	params *protocol.InitializeParams,
) (any, error) {
	capabilities := ls.handler.CreateServerCapabilities()

	return protocol.InitializeResult{
		Capabilities: capabilities,
		ServerInfo: &protocol.InitializeResultServerInfo{
			Name:    lsName,
			Version: &version,
		},
	}, nil
}

func (ls *LanguageServer) initialized(
	context *glsp.Context,
	params *protocol.InitializedParams,
) error {
	log.Println("Server initialized")
	return nil
}

func (ls *LanguageServer) shutdown(context *glsp.Context) error {
	log.Println("Server shutting down")
	protocol.SetTraceValue(protocol.TraceValueOff)
	return nil
}

func (ls *LanguageServer) setTrace(context *glsp.Context, params *protocol.SetTraceParams) error {
	protocol.SetTraceValue(params.Value)
	log.Printf("Trace set to: %s", params.Value)
	return nil
}

func (ls *LanguageServer) executeCommand(
	context *glsp.Context,
	params *protocol.ExecuteCommandParams,
) (interface{}, error) {
	// log.Printf("State is %s", ls.state)
	log.Println("Hello World")
	return nil, nil
}
