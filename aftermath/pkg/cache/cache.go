package cache

import (
	"aftermath/bindings"
	"aftermath/internal/database"
	"aftermath/internal/parser"
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

// Utilizes the CPU for `n` milliseconds.
func utilizeCPU(n int) {
	// Record the start time
	start := time.Now()

	// Run the CPU-intensive task for approximately `n` milliseconds
	for time.Since(start) < time.Duration(n)*time.Microsecond {
		// Perform some work, here we use a simple loop to generate some load
		_ = 1 + 1 // A trivial operation, but it consumes CPU time
	}
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
		if info.IsDir() {
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

	for u := range updateChan {
		_ = u.zettel
		m := u.metadata

		fmt.Printf("Updating %s", m.Path)

		content, err := os.ReadFile(m.Path)
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		refs, err := parser.GetReferences(content)
		if err != nil {
			fmt.Println("Error:", err)
		}

		for r := range refs {
			fmt.Printf("Adding ref to %d", r)
			// TODO: actually add refs
		}

		db.CreateZettel(
			database.Zettel{
				LastUpdated: m.LastModified.Unix(),
				Path:        m.Path,
				Checksum:    []byte("hu"),
			},
		)
	}
}

// UpdateIncremental is the main function to set up the directory walking and processing routines.
func UpdateIncremental(dir string) {
	// Channel to send file metadata for concurrent processing
	fileMetadataChan := make(chan FileMetadata, 10000)

	// WaitGroup to wait for all goroutines
	var wg sync.WaitGroup

	// Start the directory walking goroutine
	wg.Add(1)
	go findUpdates(fileMetadataChan, &wg)

	// Start the metadata processing goroutine
	wg.Add(1)
	go func() {
		walkDirectory(dir, fileMetadataChan, &wg)
		close(fileMetadataChan)
	}()

	// Wait for all goroutines to finish
	wg.Wait()
}
