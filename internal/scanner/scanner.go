// scanner is used to scan a directory for notes.
package scanner

import (
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
)

func Scan(
	root string,
	skip func(path string, info fs.FileInfo) bool,
	callback func(path string, document []byte),
) {
	fileChan := make(chan string, 100)
	go func() {
		for f := range fileChan {
			document, err := os.ReadFile(path.Join(root, f))
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
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		notePath, _ := filepath.Rel(root, path)
		if !skip(notePath, info) {
			fileChan <- notePath
		}

		return nil
	})
}
