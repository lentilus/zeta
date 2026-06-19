package server

import (
	"fmt"
	"io/fs"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"zeta/internal/cache"
	"zeta/internal/config"
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

    // Pre‑create parser pools for every configured language
    parsers := make(map[string]*parser.ParserPool)
    for _, lang := range uniqueLanguages(s.config) {
        parsers[lang] = parser.NewParserPool(10, lang)
    }

    seenNotes := make(map[cache.Path]struct{})
    now := time.Now()
    startTime := now  // <-- start of indexing

    var (
        totalCount  int32
        todoCount   int32
        cachedCount int32
        wg          sync.WaitGroup
    )

    skipFunc := func(absolutepath string, info fs.FileInfo) bool {
        note, err := resolver.Resolve(absolutepath)
        if err != nil {
            return true
        }
        seenNotes[note.CachePath] = struct{}{}
        atomic.AddInt32(&totalCount, 1)
        if !s.cache.GetSaveTime(note.CachePath).After(info.ModTime()) {
            atomic.AddInt32(&todoCount, 1)
            return false
        }
        return true
    }

    callbackFunc := func(absolutepath string, document []byte) {
        note, err := resolver.Resolve(absolutepath)
        if err != nil {
            log.Printf("Unexpected error resolving %v", err)
            return
        }
        cacheQueue <- documentScan{note: note, content: document}
    }

    // ----- Worker pool -----
    const numWorkers = 8 // or runtime.NumCPU()
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for d := range cacheQueue {
                pool := parsers[d.note.Format]
                links, meta := pool.ParseAndExtractLinksAndMeta(d.note, d.content)
                if err := s.cache.SaveNote(d.note.CachePath, links, meta, now); err != nil {
                    log.Println(err)
                }
                atomic.AddInt32(&cachedCount, 1)
            }
        }()
    }

    // ----- Progress reporter (optional) -----
    progress := NewProgressReporter(context)
    progress.Begin("Caching", "…")
    done := make(chan struct{})
    go func() {
        ticker := time.NewTicker(200 * time.Millisecond)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                c := atomic.LoadInt32(&cachedCount)
                t := atomic.LoadInt32(&todoCount)
                progress.Report(fmt.Sprintf("Caching %d/%d notes", c, t))
            case <-done:
                return
            }
        }
    }()

    // Walk the filesystem and feed the queue
    scanner.Scan(s.config.Root, skipFunc, callbackFunc)

    close(cacheQueue) // no more files
    wg.Wait()         // all workers done
    close(done)       // stop progress reporter

    // Purge stale notes
    for _, note := range s.cache.GetPaths() {
        if _, ok := seenNotes[note]; !ok {
            s.cache.DeleteNote(note)
        }
    }

    // ----- Speed summary -----
    elapsed := time.Since(startTime)
    todo := atomic.LoadInt32(&todoCount)
    notesPerSec := float64(todo) / elapsed.Seconds()
    msg := fmt.Sprintf("Indexed %d notes in %.2f seconds (%.1f notes/sec)", todo, elapsed.Seconds(), notesPerSec)

    progress.End(msg)                          // final progress message
    log.Println(msg)

    return nil
}

// helper: extract unique language names from config
func uniqueLanguages(cfg config.Config) []string {
    seen := map[string]bool{}
    for _, lang := range cfg.Extensions {
        seen[lang] = true
    }
    langs := make([]string, 0, len(seen))
    for l := range seen {
        langs = append(langs, l)
    }
    return langs
}
