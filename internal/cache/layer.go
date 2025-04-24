package cache

import (
	"fmt"
	"sync"
)

type layer interface {
	upsert(note note, links []Link) error
	delete(path Path) error
	paths() ([]Path, error)
	forwardLinks(path Path) ([]Link, error)
	backLinks(path Path) ([]Link, error)
	info(path Path) (note, bool)
}

// implements cacheLayer
type hashmapLayer struct {
	fLinks map[Path][]Link
	bLinks map[Path][]Link
	notes  map[Path]note
	mu     sync.RWMutex
}

// newHashmapLayer initializes and returns a new instance of mapCacheLayer.
func newHashmapLayer() *hashmapLayer {
	return &hashmapLayer{
		fLinks: make(map[Path][]Link),
		bLinks: make(map[Path][]Link),
		notes:  make(map[Path]note),
		mu:     sync.RWMutex{},
	}
}

// helper function
func (l *hashmapLayer) deleteLinks(path Path) error {
	// Remove old outgoing links and corresponding backlinks
	for _, link := range l.fLinks[path] {
		backLinks := l.bLinks[link.Tgt]
		var updatedBackLinks []Link
		for _, backLink := range backLinks {
			if backLink.Src != path {
				updatedBackLinks = append(updatedBackLinks, backLink)
			}
		}
		l.bLinks[link.Tgt] = updatedBackLinks
	}
	delete(l.fLinks, path)
	return nil
}

// helper function
func (l *hashmapLayer) overrideLinks(path Path, links []Link) error {
	// remove old Links
	err := l.deleteLinks(path)
	if err != nil {
		return err
	}

	// Set new forward links
	l.fLinks[path] = links

	// Set corresponding backlinks
	for _, link := range links {
		l.bLinks[link.Tgt] = append(l.bLinks[link.Tgt], link)
	}
	return nil
}

func (l *hashmapLayer) upsert(note note, links []Link) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Validate each link.
	for _, link := range links {
		// Check that the source of the link matches the note's path.
		if link.Src != note.Path {
			return fmt.Errorf(
				"invalid link: src %s does not match note path %s",
				link.Src,
				note.Path,
			)
		}
		// If the target is not the note itself, check that it exists.
		if link.Tgt != note.Path {
			if _, exists := l.notes[link.Tgt]; !exists {
				return fmt.Errorf(
					"invalid link: target note %s does not exist",
					link.Tgt,
				)
			}
		}
	}

	// Insert or update the note.
	l.notes[note.Path] = note

	// Update forward and backward links by overriding existing links.
	return l.overrideLinks(note.Path, links)
}

// delete removes a note and its forward links from the cache,
// but returns an error if the note has any backlinks.
func (l *hashmapLayer) delete(path Path) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if there are any backlinks for this note.
	if backlinks, exists := l.bLinks[path]; exists && len(backlinks) > 0 {
		return fmt.Errorf("cannot delete note %s: note has backlinks", path)
	}

	// Remove the note from the cache.
	delete(l.notes, path)

	// Remove outgoing forward links only.
	delete(l.fLinks, path)

	return nil
}

// note retrieves a note by its path.
func (l *hashmapLayer) note(path Path) note {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if n, ok := l.notes[path]; ok {
		return n
	}
	return note{}
}

// paths returns a slice containing all the note paths in the cache.
func (l *hashmapLayer) paths() ([]Path, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var ps []Path
	for p := range l.notes {
		ps = append(ps, p)
	}
	return ps, nil
}

// forwardLinks returns all links that originate from the note at the given path.
func (l *hashmapLayer) forwardLinks(path Path) ([]Link, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if links, ok := l.fLinks[path]; ok {
		return links, nil
	}
	return nil, nil
}

// backLinks returns all links that point to the note at the given path.
func (l *hashmapLayer) backLinks(path Path) ([]Link, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if links, ok := l.bLinks[path]; ok {
		return links, nil
	}
	return nil, nil
}

// info retrieves a note and a boolean indicating if it exists.
func (l *hashmapLayer) info(path Path) (note, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	note, exists := l.notes[path]
	return note, exists
}
