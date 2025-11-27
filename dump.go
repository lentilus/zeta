package main

import (
	"fmt"
	"io/fs"
	"os"
	"time"
	"zeta/internal/cache"
	"zeta/internal/config"
	"zeta/internal/parser"
	"zeta/internal/resolver"
	"zeta/internal/scanner"
)

func runDump(configPath string) error {
	f, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer f.Close()
	cfg, err := config.LoadFromJSON(f)
	if err != nil {
		return err
	}

	resolver.Configure(cfg.Root, cfg.Extensions)

	c := cache.NewCache()
	now := time.Now()

	langSet := make(map[string]bool)
	for _, lang := range cfg.Extensions {
		langSet[lang] = true
	}
	parserPools := make(map[string]*parser.ParserPool, len(langSet))
	for lang := range langSet {
		parserPools[lang] = parser.NewParserPool(10, lang)
	}
	defer func() {
		for _, pool := range parserPools {
			pool.Close()
		}
	}()

	skip := func(path string, info fs.FileInfo) bool {
		return false // always re‑scan when dumping
	}

	callback := func(path string, data []byte) {
		note, err := resolver.Resolve(path)
		if err != nil {
			return
		}
		pool, ok := parserPools[note.Format]
		if !ok {
			fmt.Fprintf(os.Stderr, "no parser pool for format %s of note %s\n", note.Format, note.AbsolutePath)
			return
		}
		links, meta := pool.ParseAndExtractLinksAndMeta(note, data)
		if err := c.SaveNote(note.CachePath, links, meta, now); err != nil {
			fmt.Fprintf(os.Stderr, "failed to save note %s: %v\n", note.CachePath, err)
		}
	}

	scanner.Scan(cfg.Root, skip, callback)
	fmt.Print(string(c.Dump()))
	return nil
}
