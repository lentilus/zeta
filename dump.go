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

	resolver.Configure(cfg.Root, cfg.SelectRegex)

	c := cache.NewCache()
	now := time.Now()
	skip := func(path string, info fs.FileInfo) bool {
		return false // always re-scan for dump
	}
	parserPool := parser.NewParserPool(10)
	callback := func(path string, data []byte) {
		note, err := resolver.Resolve(path)
		if err != nil {
			return
		}
		matches, _ := parserPool.ParseAndQuery(data, []byte(cfg.Query))
		links := resolver.ExtractLinks(note, matches, data)
		_ = c.SaveNote(note.CachePath, links, now)
	}
	scanner.Scan(cfg.Root, skip, callback)
	fmt.Print(string(c.Dump()))
	return nil
}
