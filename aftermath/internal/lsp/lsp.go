package lsp

import (
	"log"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

const lsName = "aftermath"

var version string = "0.0.1"

func NewHandler() *protocol.Handler {
	var handler protocol.Handler
	handler = protocol.Handler{
		Initialize:              initializeClosure(&handler),
		Initialized:             initialized,
		Shutdown:                shutdown,
		SetTrace:                setTrace,
		WorkspaceExecuteCommand: workspaceExecuteCommand,
	}
	return &handler
}

func initializeClosure(
	handler *protocol.Handler,
) func(*glsp.Context, *protocol.InitializeParams) (any, error) {
	return func(context *glsp.Context, params *protocol.InitializeParams) (any, error) {
		capabilities := handler.CreateServerCapabilities()

		capabilities.ExecuteCommandProvider = &protocol.ExecuteCommandOptions{
			Commands: []string{
				"test",
			},
		}

		return protocol.InitializeResult{
			Capabilities: capabilities,
			ServerInfo: &protocol.InitializeResultServerInfo{
				Name:    lsName,
				Version: &version,
			},
		}, nil
	}
}

func initialized(context *glsp.Context, params *protocol.InitializedParams) error {
	return nil
}

func shutdown(context *glsp.Context) error {
	protocol.SetTraceValue(protocol.TraceValueOff)
	return nil
}

func setTrace(context *glsp.Context, params *protocol.SetTraceParams) error {
	protocol.SetTraceValue(params.Value)
	return nil
}

func workspaceExecuteCommand(
	context *glsp.Context,
	params *protocol.ExecuteCommandParams,
) (interface{}, error) {
	log.Println("Called Execute Command")
	return nil, nil
}
