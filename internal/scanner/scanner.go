// scanner is used to scan a directory for notes.
package scanner

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

func Scan(
	root string,
	skip func(path string, info fs.FileInfo) bool,
	callback func(path string, document []byte),
) {
	log.Println("--SCANNING--")
	fileChan := make(chan string, 100)
	go func() {
		for f := range fileChan {
			log.Printf("Processing %s", f)
			document, err := os.ReadFile(f)
			if err != nil {
				log.Println("Error reading file:", err)
				continue
			}
			notePath, _ := filepath.Rel(root, f)
			callback(notePath, document)
		}
	}()

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			log.Print(err)
			return nil
		}
		if d.IsDir() {
			log.Printf("%s is a directory", path)
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		if !skip(path, info) {
			fileChan <- path
		}

		return nil
	})
}
