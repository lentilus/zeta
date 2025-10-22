package server

import (
	"fmt"
	"io/fs"
	"log"
	"sync/atomic"
	"time"

	"zeta/internal/cache"
	"zeta/internal/parser"
	"zeta/internal/resolver"
	"zeta/internal/scanner"

	"github.com/tliron/glsp"
	protocol "github.com/tliron/glsp/protocol_3_16"
)


type ProgressParamsFixed struct {
    Token any `json:"token"`
    Value any `json:"value"`
}

func indexNotes(
	rootPath string,
	context *glsp.Context,
	noteCache cache.Cache,
	tsQuery string,
) error {
	parsers := parser.NewParserPool(10)
	seenNotes := map[cache.Path]struct{}{}
	now := time.Now()

	var totalCount int32
	var changedCount int32
	var processedCount int32

	progressToken := "indexing-progress"

	// TODO: this may also suffer from token unmarshal bug in glsp
	context.Call("window/workDoneProgress/create", protocol.WorkDoneProgressCreateParams{Token: protocol.ProgressToken{Value: progressToken}}, nil)

	sendProgress := func(kind string) {
		msg := fmt.Sprintf("updating %d/%d [total %d]", atomic.LoadInt32(&processedCount),
			atomic.LoadInt32(&changedCount),
			atomic.LoadInt32(&totalCount))

		var value any
		switch kind {
		case "begin":
			value = protocol.WorkDoneProgressBegin{
				Kind:        "begin",
				Title:       "Cache",
				Cancellable: &protocol.False,
				Message:     &msg,
			}
		case "report":
			value = protocol.WorkDoneProgressReport{
				Kind:    "report",
				Message: &msg,
			}
		case "end":
			value = protocol.WorkDoneProgressEnd{
				Kind:    "end",
				Message: &msg,
			}
		}

		context.Notify("$/progress", ProgressParamsFixed{
			Token: progressToken,
			Value: value,
		})
	}

	skipFunc := func(absolutepath string, info fs.FileInfo) bool {
		note, err := resolver.Resolve(absolutepath)
		if err != nil {
			return true
		}
		seenNotes[note.CachePath] = struct{}{}

		atomic.AddInt32(&totalCount, 1)
		lastSeen := noteCache.GetSaveTime(note.CachePath)
		hasNotChanged := lastSeen.After(info.ModTime())
		if !hasNotChanged {
			atomic.AddInt32(&changedCount, 1)
		}
		return hasNotChanged
	}

	callbackFunc := func(absolutepath string, document []byte) {
		defer atomic.AddInt32(&processedCount, 1)

		note, err := resolver.Resolve(absolutepath)
		if err != nil {
			log.Printf("Unexpected error resolving %v", err)
			return
		}

		nodes, err := parsers.ParseAndQuery(document, []byte(tsQuery))
		if err != nil {
			log.Printf("Unexpected error parsing %v", err)
			return
		}

		links, meta := resolver.ExtractLinksAndMeta(note, nodes, document)
		if err := noteCache.SaveNote(note.CachePath, links, meta, now); err != nil {
			log.Println(err)
		}
	}

	go func() {
		stopCh := make(chan struct{})
		done := make(chan struct{}) // signal when ticker is done
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		// progress reporting
		go func() {
			defer close(done)
		    sendProgress("begin")
			for {
				select {
				case <-stopCh:
					sendProgress("end")
					return
				case <-ticker.C:
					log.Println("Sending report")
					sendProgress("report")
				}
			}
		}()

		scanner.Scan(rootPath, skipFunc, callbackFunc)

		for _, note := range noteCache.GetPaths() {
			if _, ok := seenNotes[note]; !ok {
				noteCache.DeleteNote(note)
			}
		}

		close(stopCh)
		<-done
	}()

	return nil
}
