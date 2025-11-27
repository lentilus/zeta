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

type documentScan struct{
	note resolver.Note
	content []byte
}

func (s *Server) indexNotes(context *glsp.Context) error {
	cacheQueue := make(chan documentScan, 100)
	embedQueue := make(chan documentScan, 100)

	// TODO!
	// parsers := parser.NewParserPool(10, "markdown")
	parsers := make(map[string]*parser.ParserPool)
	seenNotes := map[cache.Path]struct{}{}
	now := time.Now()

	var (
		totalCount int32
		todoCount int32
		cachedCount int32
		embeddedCount int32
	)


	skipFunc := func(absolutepath string, info fs.FileInfo) bool {
		note, err := resolver.Resolve(absolutepath)
		if err != nil {
			return true
		}
		seenNotes[note.CachePath] = struct{}{}

		atomic.AddInt32(&totalCount, 1)
		lastSeen := s.cache.GetSaveTime(note.CachePath)
		hasNotChanged := lastSeen.After(info.ModTime())
		if !hasNotChanged {
			atomic.AddInt32(&todoCount, 1)
		}
		return hasNotChanged
	}

	callbackFunc := func(absolutepath string, document []byte) {
		note, err := resolver.Resolve(absolutepath)
		if err != nil {
			log.Printf("Unexpected error resolving %v", err)
			return
		}

		doc := documentScan{
			note: note,
			content: document,
		}

		cacheQueue<-doc
		embedQueue<-doc
	}


	go func(){
		// cache new notes
		progress := NewProgressReporter(context)
		var msg string

		progress.Begin("Caching", msg)


		// TODO: make this concurrent
	    for d := range cacheQueue {
			format := d.note.Format
			var pool *parser.ParserPool
			if parsers[format] != nil {
				pool = parsers[format]
			} else {
				pool = parser.NewParserPool(10, format)
				parsers[format] = pool
			}

			links, meta := pool.ParseAndExtractLinksAndMeta(d.note, d.content)
			if err := s.cache.SaveNote(d.note.CachePath, links, meta, now); err != nil {
				log.Println(err)
			}

		    atomic.AddInt32(&cachedCount, 1)

			msg = fmt.Sprintf(
				"updating %d/%d [total %d]",
				atomic.LoadInt32(&cachedCount),
			    atomic.LoadInt32(&todoCount),
			    atomic.LoadInt32(&totalCount))
			progress.Report(msg)
	    }

		progress.End(msg)

		// purge stale notes
		for _, note := range s.cache.GetPaths() {
			if _, ok := seenNotes[note]; !ok {
				s.cache.DeleteNote(note)
			}
		}
	}()

	// Embedding
	go func() {
		progress := NewProgressReporter(context)
		var msg string

		progress.Begin("Embedding", msg)

		// NOTE: dont make this concurrent to save ressources
	    for d := range embedQueue {
	    	log.Printf("embedding %s", d.note.CachePath)

		    atomic.AddInt32(&embeddedCount, 1)

			msg = fmt.Sprintf(
				"updating %d/%d [total %d]",
				atomic.LoadInt32(&embeddedCount),
			    atomic.LoadInt32(&todoCount),
			    atomic.LoadInt32(&totalCount))
			progress.Report(msg)

			// TODO: check argument order
			s.embedder.Embed(d.note.CachePath, string(d.content))
	    }
		progress.End(msg)

		// TODO: remove stale embeddings
	}()

	scanner.Scan(s.config.Root, skipFunc, callbackFunc)

	close(cacheQueue)
	close(embedQueue)

	return nil
}
