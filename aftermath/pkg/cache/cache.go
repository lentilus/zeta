package cache

import (
	"aftermath/bindings"
	"aftermath/internal/database"
	"aftermath/internal/parser"
	"aftermath/internal/utils"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
)

// FileMetadata holds the file path and its last modified timestamp.
type FileMetadata struct {
	Path         string
	LastModified time.Time
}

type ZettelUpdate struct {
	metadata FileMetadata
	zettel   database.Zettel
}

// fileNameFilter takes a filename and returns true if it is a zettel, false if it is not.
func fileNameFilter(name string) bool {
	return name[len(name)-4:] == ".typ"
}

// UpdateIncremental is the main function to set up the directory walking and processing routines.
func UpdateIncremental(dir string) {
	fileMetadataChan := make(chan FileMetadata, 10000)
	var wg sync.WaitGroup

	// Start directory walking
	wg.Add(1)
	go findUpdates(fileMetadataChan, &wg)

	// Start metadata processing
	wg.Add(1)
	go func() {
		walkDirectory(dir, fileMetadataChan, &wg)
		close(fileMetadataChan)
	}()

	// Wait for all goroutines to finish
	wg.Wait()
}

// walkDirectory walks through the directory, sending file metadata to a channel.
func walkDirectory(dir string, fileMetadataChan chan<- FileMetadata, wg *sync.WaitGroup) {
	defer wg.Done()

	// Walk the directory and process each file.
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("error accessing path %q: %v\n", path, err)
			return nil
		}

		if !fileNameFilter(path) {
			return nil
		}
		// If it is a file, send its metadata to the channel.
		fileMetadataChan <- FileMetadata{
			Path:         path,
			LastModified: info.ModTime(),
		}
		return nil
	})

	if err != nil {
		fmt.Printf("error walking the directory %q: %v\n", dir, err)
	}
}

// findUpdates reads file metadata from the channel and processes it.
func findUpdates(fileMetadataChan <-chan FileMetadata, wg *sync.WaitGroup) error {
	defer wg.Done()

	db, err := database.NewDB("/home/lentilus/typstest/db.sqlite")
	if err != nil {
		return err
	}
	zettels, err := db.GetAll()
	fmt.Printf("%d zettels stored in db\n", len(zettels))
	if err != nil {
		return err
	}
	_ = sitter.NewLanguage(bindings.Language())

	processorChan := make(chan ZettelUpdate, 10000)

	var processWg sync.WaitGroup
	processWg.Add(1)
	go processUpdates(db, processorChan, &processWg)

	for metadata := range fileMetadataChan {
		z, exists := zettels[metadata.Path]

		// Check if file has not changed
		if exists {
			delete(zettels, metadata.Path)
			if z.LastUpdated >= metadata.LastModified.Unix() {
				continue
			}
		}

		processorChan <- ZettelUpdate{zettel: z, metadata: metadata}
	}
	close(processorChan)

	// Delete all zettels left in zettels (the deleted ones) from DB
	var ids []int
	for _, z := range zettels {
		ids = append(ids, z.ID)
	}
	db.DeleteZettels(ids)

	fmt.Print("deleted old zettels:")
	fmt.Println(ids)

	processWg.Wait()
	return nil
}

func processUpdates(db *database.DB, updateChan <-chan ZettelUpdate, wg *sync.WaitGroup) {
	defer wg.Done()
	parser := parser.NewParser()
	defer parser.CloseParser()

	newLinks := make(map[string][]string)

	for u := range updateChan {
		z := u.zettel
		m := u.metadata

		fmt.Printf("Updating %s\n", m.Path)

		// Read file content
		content, err := os.ReadFile(m.Path)
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		// Compare Checksums
		checksum := utils.ComputeChecksum(content)
		if bytes.Equal(checksum, z.Checksum) {
			fmt.Println("Nothing to do")
			continue
		}

		// Get references
		refs, err := parser.GetReferences(content)
		if err != nil {
			fmt.Println("Error:", err)
		} else {
			newLinks[m.Path] = refs
		}

		err = db.UpsertZettel(
			database.Zettel{
				LastUpdated: m.LastModified.Unix(),
				Path:        m.Path,
				Checksum:    checksum,
			},
		)
		if err != nil {
			fmt.Println(err)
		}
	}
}
