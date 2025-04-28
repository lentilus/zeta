// scanner is used to scan a directory for notes.
package scanner

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"zeta/internal/resolver"
)

// Scan walks the entire subtree under root. Any file or directory
// whose name begins with “.” is skipped entirely. For each remaining
// file, we apply your skip() predicate, and if that returns false
// we read the file and invoke callback(relPath, contents).
// Scan will only return once all callbacks have completed.
func Scan(
	root string,
	skip func(relPath string, info fs.FileInfo) bool,
	callback func(relPath string, document []byte),
) {
	fileCh := make(chan string, 100)
	var wg sync.WaitGroup

	// worker goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for path := range fileCh {
			data, err := os.ReadFile(path)
			if err != nil {
				log.Println("scanner: read error:", path, err)
				continue
			}
			callback(path, data)
		}
	}()

	log.Printf("scanner: starting WalkDir at %q", root)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Println("scanner: walk error:", err)
			return nil
		}

		if d.IsDir() {
			if resolver.IngoreDir(path) {
				log.Printf("Skipping %q", path)
				return fs.SkipDir
			}
			log.Printf("scanner: descending into %q", path)
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		if skip(path, info) {
			return nil
		}

		// enqueue for reading
		fileCh <- path
		return nil
	})
	if err != nil {
		log.Println("scanner: WalkDir finished with error:", err)
	}

	// no more files to send
	close(fileCh)
	// wait for the worker to finish consuming and calling back
	wg.Wait()
}
