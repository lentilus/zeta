package cache

import (
	"fmt"
	"sync"
)

type cacheLayer interface {
	upsert(note Note, links []Link) error
	delete(path Path) error
	info(path Path) (Note, bool)
	paths() ([]Path, error)
	forwardLinks(path Path) ([]Link, error)
	backLinks(path Path) ([]Link, error)
}

// implements cacheLayer
type hashmapLayer struct {
	fLinks map[Path][]Link
	bLinks map[Path][]Link
	notes  map[Path]Note
	mu     sync.RWMutex
}

// helper function
func (hl *hashmapLayer) deleteLinks(path Path) error {
	// Remove old outgoing links and corresponding backlinks
	for _, link := range hl.fLinks[path] {
		backLinks := hl.bLinks[link.Tgt]
		var updatedBackLinks []Link
		for _, backLink := range backLinks {
			if backLink.Src != path {
				updatedBackLinks = append(updatedBackLinks, backLink)
			}
		}
		hl.bLinks[link.Tgt] = updatedBackLinks
	}
	delete(hl.fLinks, path)
	return nil
}

// helper function
func (hl *hashmapLayer) overrideLinks(path Path, links []Link) error {
	// remove old Links
	err := hl.deleteLinks(path)
	if err != nil {
		return err
	}

	// Set new forward links
	hl.fLinks[path] = links

	// Set corresponding backlinks
	for _, link := range links {
		hl.bLinks[link.Tgt] = append(hl.bLinks[link.Tgt], link)
	}
	return nil
}

func (hl *hashmapLayer) upsert(note Note, links []Link) error {
	hl.mu.Lock()
	defer hl.mu.Unlock()

	// Validate each link.
	for _, link := range links {
		// Check that the source of the link matches the note's path.
		if link.Src != note.Path {
			return fmt.Errorf(
				"invalid link %s: src %s does not match note path %s",
				link.Ref,
				link.Src,
				note.Path,
			)
		}
		// If the target is not the note itself, check that it exists.
		if link.Tgt != note.Path {
			if _, exists := hl.notes[link.Tgt]; !exists {
				return fmt.Errorf(
					"invalid link %s: target note %s does not exist",
					link.Ref,
					link.Tgt,
				)
			}
		}
	}

	// Insert or update the note.
	hl.notes[note.Path] = note

	// Update forward and backward links by overriding existing links.
	return hl.overrideLinks(note.Path, links)
}

// delete removes a note and its forward links from the cache,
// but returns an error if the note has any backlinks.
func (hl *hashmapLayer) delete(path Path) error {
	hl.mu.Lock()
	defer hl.mu.Unlock()

	// Check if there are any backlinks for this note.
	if backlinks, exists := hl.bLinks[path]; exists && len(backlinks) > 0 {
		return fmt.Errorf("cannot delete note %s: note has backlinks", path)
	}

	// Remove the note from the cache.
	delete(hl.notes, path)

	// Remove outgoing forward links only.
	delete(hl.fLinks, path)

	return nil
}

// note retrieves a note by its path.
func (hl *hashmapLayer) note(path Path) Note {
	hl.mu.RLock()
	defer hl.mu.RUnlock()

	if n, ok := hl.notes[path]; ok {
		return n
	}
	return Note{}
}

// paths returns a slice containing all the note paths in the cache.
func (hl *hashmapLayer) paths() ([]Path, error) {
	hl.mu.RLock()
	defer hl.mu.RUnlock()

	var ps []Path
	for p := range hl.notes {
		ps = append(ps, p)
	}
	return ps, nil
}

// forwardLinks returns all links that originate from the note at the given path.
func (hl *hashmapLayer) forwardLinks(path Path) ([]Link, error) {
	hl.mu.RLock()
	defer hl.mu.RUnlock()

	if links, ok := hl.fLinks[path]; ok {
		return links, nil
	}
	return nil, nil
}

// backLinks returns all links that point to the note at the given path.
func (hl *hashmapLayer) backLinks(path Path) ([]Link, error) {
	hl.mu.RLock()
	defer hl.mu.RUnlock()

	if links, ok := hl.bLinks[path]; ok {
		return links, nil
	}
	return nil, nil
}
