package store

import (
	"aftermath/internal/bibliography"
	"aftermath/internal/cache/database"
	"aftermath/internal/parser"
	"aftermath/internal/utils"
	"bytes"
	"fmt"
	"log"
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

type ZettelStore struct {
	root string
	db   *database.DB
	bib  *bibliography.Bibliography
}

func NewZettelStore(
	root string,
	db *database.DB,
	bib *bibliography.Bibliography,
) *ZettelStore {
	return &ZettelStore{root: root, db: db, bib: bib}
}

// fileNameFilter takes a filename and returns true if it is a zettel, false if it is not.
func fileNameFilter(name string) bool {
	return name[len(name)-4:] == ".typ"
}

// UpdateIncremental is the main function to set up the directory walking and processing routines.
func (k *ZettelStore) UpdateIncremental() error {
	fileMetadataChan := make(chan FileMetadata, 10000)
	var wg sync.WaitGroup

	// Start directory walking
	wg.Add(1)
	go func() {
		if err := k.findUpdates(fileMetadataChan, &wg); err != nil {
			fmt.Printf("Error finding updates: %v\n", err)
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

	err := k.bib.Regenerate(k.root)
	if err != nil {
		return err
	}
	return nil
}

// walkDirectory walks through the directory, sending file metadata to a channel.
func walkDirectory(dir string, fileMetadataChan chan<- FileMetadata, wg *sync.WaitGroup) {
	defer wg.Done()

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("error accessing path %q: %v\n", path, err)
			return nil
		}

		if !fileNameFilter(path) {
			return nil
		}
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
func (k *ZettelStore) findUpdates(fileMetadataChan <-chan FileMetadata, wg *sync.WaitGroup) error {
	defer wg.Done()

	zettels, err := k.db.GetAll()
	if err != nil {
		return err
	}
	processorChan := make(chan ZettelUpdate, 10000)

	var processWg sync.WaitGroup
	processWg.Add(1)
	go k.processUpdates(processorChan, &processWg)

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
	log.Printf("Deleting %d zettels from cache.", len(zettels))
	for _, z := range zettels {
		ids = append(ids, z.ID)
	}
	k.db.DeleteZettels(ids)

	processWg.Wait()
	return nil
}

func (k *ZettelStore) processUpdates(
	updateChan <-chan ZettelUpdate,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	parser := parser.NewParser()
	defer parser.CloseParser()

	newLinks := make(map[string][]string)
	updatedCount := 0

	for u := range updateChan {
		z := u.zettel
		m := u.metadata

		content, err := os.ReadFile(m.Path)
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		checksum := utils.ComputeChecksum(content)
		if bytes.Equal(checksum, z.Checksum) {
			fmt.Println("Nothing to do")
			continue
		}

		refs, err := parser.GetReferences(content)
		if err != nil {
			fmt.Println("Error:", err)
		} else {
			newLinks[m.Path] = refs
		}

		err = k.db.UpsertZettel(
			database.Zettel{
				LastUpdated: m.LastModified.Unix(),
				Path:        m.Path,
				Checksum:    checksum,
			},
		)
		if err != nil {
			fmt.Println(err)
		} else {
			updatedCount++
		}
	}
	err := k.updateLinks(newLinks)
	if err != nil {
		fmt.Println(err)
	}

	if updatedCount > 0 {
		log.Printf("Upserted %d zettels", updatedCount)
	} else {
		log.Println("No upserts.")
	}
}

func (k *ZettelStore) updateLinks(newLinks map[string][]string) error {
	for z, refs := range newLinks {
		err := k.db.DeleteLinks(z)
		if err != nil {
			return err
		}
		for _, ref := range refs {
			link, _ := utils.Reference2Path(ref, k.root)
			err = k.db.CreateLink(z, link)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// UpdateOne processes a single Zettel file given its path.
func (k *ZettelStore) UpdateOne(path string) error {
	// Check if file exists and get its info
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error accessing file %q: %v", path, err)
	}

	// Check if it's a valid zettel file
	if !fileNameFilter(path) {
		return fmt.Errorf("file %q is not a valid zettel file", path)
	}

	// Create FileMetadata for the file
	metadata := FileMetadata{
		Path:         path,
		LastModified: fileInfo.ModTime(),
	}

	// Get existing zettel from database if it exists
	existingZettel, dberr := k.db.GetZettel(path)
	if dberr != nil && dberr != database.ErrZettelNotFound {
		return fmt.Errorf("error retrieving zettel from database: %v", dberr)
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading file %q: %v", path, err)
	}

	// Compute checksum
	checksum := utils.ComputeChecksum(content)

	// If file hasn't changed, return early
	if dberr != database.ErrZettelNotFound && bytes.Equal(checksum, existingZettel.Checksum) {
		return nil
	}

	// Parse references
	parser := parser.NewParser()
	defer parser.CloseParser()

	refs, err := parser.GetReferences(content)
	if err != nil {
		return fmt.Errorf("error parsing references: %v", err)
	}

	// Update zettel in database
	err = k.db.UpsertZettel(
		database.Zettel{
			LastUpdated: metadata.LastModified.Unix(),
			Path:        metadata.Path,
			Checksum:    checksum,
		},
	)
	if err != nil {
		return fmt.Errorf("error updating zettel in database: %v", err)
	}

	// Update links
	err = k.db.DeleteLinks(path)
	if err != nil {
		return fmt.Errorf("error deleting old links: %v", err)
	}

	for _, ref := range refs {
		link, _ := utils.Reference2Path(ref, k.root)
		err = k.db.CreateLink(path, link)
		if err != nil {
			return fmt.Errorf("error creating link: %v", err)
		}
	}

	log.Println("Upserted 1 zettel.")

	err = k.bib.Regenerate(k.root)
	if err != nil {
		return err
	}

	return nil
}
