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
)

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
		reporter := NewProgressReporter(context)
		go func() {
			defer close(done)
			reporter.Begin("Indexing", "starting")
			for {
				select {
				case <-stopCh:
					reporter.End("")
					return
				case <-ticker.C:
					msg := fmt.Sprintf("updating %d/%d [total %d]", atomic.LoadInt32(&processedCount),
					    atomic.LoadInt32(&changedCount),
						atomic.LoadInt32(&totalCount))
					reporter.Report(msg)
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
