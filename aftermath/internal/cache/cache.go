package cache

import (
	"aftermath/internal/database"
	"aftermath/internal/parser"
	"aftermath/internal/utils"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
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

type Zettelkasten struct {
	root  string
	cache string
}

func NewZettelkasten(root string, cache string) *Zettelkasten {
	return &Zettelkasten{root: root, cache: cache}
}

// fileNameFilter takes a filename and returns true if it is a zettel, false if it is not.
func fileNameFilter(name string) bool {
	return name[len(name)-4:] == ".typ"
}

// UpdateIncremental is the main function to set up the directory walking and processing routines.
func (k *Zettelkasten) UpdateIncremental() {
	fileMetadataChan := make(chan FileMetadata, 10000)
	var wg sync.WaitGroup

	// Start directory walking
	wg.Add(1)
	go func() {
		err := k.findUpdates(fileMetadataChan, &wg)
		if err != nil {
			fmt.Println(err)
		}
	}()

	// Start metadata processing
	wg.Add(1)
	go func() {
		walkDirectory(k.root, fileMetadataChan, &wg)
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
func (k *Zettelkasten) findUpdates(fileMetadataChan <-chan FileMetadata, wg *sync.WaitGroup) error {
	defer wg.Done()

	db, err := database.NewDB(k.cache)
	if err != nil {
		return err
	}
	defer db.Close()
	zettels, err := db.GetAll()
	fmt.Printf("%d zettels stored in db\n", len(zettels))
	if err != nil {
		return err
	}
	processorChan := make(chan ZettelUpdate, 10000)

	var processWg sync.WaitGroup
	processWg.Add(1)
	go k.processUpdates(db, processorChan, &processWg)

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

	processWg.Wait()
	return nil
}

func (k *Zettelkasten) processUpdates(
	db *database.DB,
	updateChan <-chan ZettelUpdate,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	parser := parser.NewParser()
	defer parser.CloseParser()

	newLinks := make(map[string][]string)

	for u := range updateChan {
		z := u.zettel
		m := u.metadata

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
	err := k.updateLinks(db, newLinks)
	if err != nil {
		fmt.Println(err)
	}

}

func (k *Zettelkasten) updateLinks(db *database.DB, newLinks map[string][]string) error {
	for z, refs := range newLinks {
		err := db.DeleteLinks(z)
		if err != nil {
			return err
		}
		for _, ref := range refs {
			link := ref2Link(ref, k.root)
			err = db.CreateLink(z, link)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func ref2Link(ref string, base string) string {
	if len(ref) < 2 {
		return ""
	}
	file := ref[1:] // remove @ref -> ref
	return filepath.Join(base, file+".typ")
}