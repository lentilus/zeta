package cache

import (
	"fmt"
	"sync"
)

type cacheLayer interface {
	upsert(note Note, links []Link) error
	delete(path Path) error
	paths() ([]Path, error)
	forwardLinks(path Path) ([]Link, error)
	backLinks(path Path) ([]Link, error)
	info(path Path) (Note, bool)
}

// implements cacheLayer
type mapCacheLayer struct {
	fLinks map[Path][]Link
	bLinks map[Path][]Link
	notes  map[Path]Note
	mu     sync.RWMutex
}

// newMapCacheLayer initializes and returns a new instance of mapCacheLayer.
func newMapCacheLayer() *mapCacheLayer {
	return &mapCacheLayer{
		fLinks: make(map[Path][]Link),
		bLinks: make(map[Path][]Link),
		notes:  make(map[Path]Note),
		mu:     sync.RWMutex{},
	}
}

// helper function
func (cl *mapCacheLayer) deleteLinks(path Path) error {
	// Remove old outgoing links and corresponding backlinks
	for _, link := range cl.fLinks[path] {
		backLinks := cl.bLinks[link.Tgt]
		var updatedBackLinks []Link
		for _, backLink := range backLinks {
			if backLink.Src != path {
				updatedBackLinks = append(updatedBackLinks, backLink)
			}
		}
		cl.bLinks[link.Tgt] = updatedBackLinks
	}
	delete(cl.fLinks, path)
	return nil
}

// helper function
func (cl *mapCacheLayer) overrideLinks(path Path, links []Link) error {
	// remove old Links
	err := cl.deleteLinks(path)
	if err != nil {
		return err
	}

	// Set new forward links
	cl.fLinks[path] = links

	// Set corresponding backlinks
	for _, link := range links {
		cl.bLinks[link.Tgt] = append(cl.bLinks[link.Tgt], link)
	}
	return nil
}

func (cl *mapCacheLayer) upsert(note Note, links []Link) error {
	cl.mu.Lock()
	defer cl.mu.Unlock()

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
			if _, exists := cl.notes[link.Tgt]; !exists {
				return fmt.Errorf(
					"invalid link %s: target note %s does not exist",
					link.Ref,
					link.Tgt,
				)
			}
		}
	}

	// Insert or update the note.
	cl.notes[note.Path] = note

	// Update forward and backward links by overriding existing links.
	return cl.overrideLinks(note.Path, links)
}

// delete removes a note and its forward links from the cache,
// but returns an error if the note has any backlinks.
func (cl *mapCacheLayer) delete(path Path) error {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	// Check if there are any backlinks for this note.
	if backlinks, exists := cl.bLinks[path]; exists && len(backlinks) > 0 {
		return fmt.Errorf("cannot delete note %s: note has backlinks", path)
	}

	// Remove the note from the cache.
	delete(cl.notes, path)

	// Remove outgoing forward links only.
	delete(cl.fLinks, path)

	return nil
}

// note retrieves a note by its path.
func (cl *mapCacheLayer) note(path Path) Note {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	if n, ok := cl.notes[path]; ok {
		return n
	}
	return Note{}
}

// paths returns a slice containing all the note paths in the cache.
func (hl *mapCacheLayer) paths() ([]Path, error) {
	hl.mu.RLock()
	defer hl.mu.RUnlock()

	var ps []Path
	for p := range hl.notes {
		ps = append(ps, p)
	}
	return ps, nil
}

// forwardLinks returns all links that originate from the note at the given path.
func (cl *mapCacheLayer) forwardLinks(path Path) ([]Link, error) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	if links, ok := cl.fLinks[path]; ok {
		return links, nil
	}
	return nil, nil
}

// backLinks returns all links that point to the note at the given path.
func (cl *mapCacheLayer) backLinks(path Path) ([]Link, error) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	if links, ok := cl.bLinks[path]; ok {
		return links, nil
	}
	return nil, nil
}

// info retrieves a note and a boolean indicating if it exists.
func (cl *mapCacheLayer) info(path Path) (Note, bool) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	note, exists := cl.notes[path]
	return note, exists
}
