package server

import (
	"fmt"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

// TODO: make tokenCounter concurrency safe
var tokenCounter uint = 0

var prefix string = "zeta-progress-"

type progressReporter struct{
	token string
	context *glsp.Context
}

type ProgressParamsFixed struct {
    Token string `json:"token"`
    Value any `json:"value"`
}

type WorkDoneProgressCreateParamsFixed struct {
	Token string `json:"token"`
}

func NewProgressReporter(context *glsp.Context) *progressReporter {
	tokenCounter ++
	token := fmt.Sprintf("%s%d", prefix, tokenCounter)

    context.Call(
		"window/workDoneProgress/create",
	 	WorkDoneProgressCreateParamsFixed{
			Token: token,
		}, nil)

	return &progressReporter{
		token: token,
		context: context,
	}
}

func (pr *progressReporter) sendProgress(value any) {
	pr.context.Notify("$/progress", ProgressParamsFixed{
		Token: pr.token,
		Value: value,
	})
}

func (pr *progressReporter) Begin(title, msg string) {
	val := protocol.WorkDoneProgressBegin{
		Kind:        "begin",
		Title:       title,
		Cancellable: &protocol.False,
		Message:     &msg,
	}
	pr.sendProgress(val)
}

func (pr *progressReporter) Report(msg string) {
	val := protocol.WorkDoneProgressBegin{
		Kind:    "report",
		Message: &msg,
	}
	pr.sendProgress(val)
}

func (pr *progressReporter) End(msg string) {
	val := protocol.WorkDoneProgressBegin{
		Kind:    "end",
		Message: &msg,
	}
	pr.sendProgress(val)
}
