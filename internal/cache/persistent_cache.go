package cache

import (
	"fmt"
	"sync"
)

type noteID uint // for identification in cytoscape only.

// PersistentCache holds the temporary and persistent cache layers, as well as index and subscriber data.
type PersistentCache struct {
	tmpLayer    cacheLayer
	pstLayer    cacheLayer
	idx         map[Path]noteID
	idxCounter  noteID
	subscribers []chan Event
	subMu       sync.RWMutex
	mu          sync.RWMutex
}

// NewPersistentCache initializes and returns a new instance of PersistentCache.
func NewPersistentCache() *PersistentCache {
	return &PersistentCache{
		tmpLayer:    newMapCacheLayer(),
		pstLayer:    newMapCacheLayer(),
		idx:         make(map[Path]noteID),
		idxCounter:  0,
		subscribers: []chan Event{},
		mu:          sync.RWMutex{},
	}
}

// getTargets builds a map from target paths for the given links,
// ensuring that all links have the same source path.
func getTargets(path Path, links []Link) (map[Path]struct{}, error) {
	m := make(map[Path]struct{}, len(links))
	for _, link := range links {
		if link.Src != path {
			return nil, fmt.Errorf(
				"invalid link: src %s does not match note path %s",
				link.Src,
				path,
			)
		}
		m[link.Tgt] = struct{}{}
	}
	return m, nil
}

// diff computes the difference in outgoing links from a path.
// It returns the targets only in a and only in b.
func diff(path Path, a, b []Link) ([]Path, []Path, error) {
	fromA, err := getTargets(path, a)
	if err != nil {
		return nil, nil, err
	}
	fromB, err := getTargets(path, b)
	if err != nil {
		return nil, nil, err
	}

	var onlyA, onlyB []Path
	// Targets in a but not in b.
	for tgt := range fromA {
		if _, found := fromB[tgt]; !found {
			onlyA = append(onlyA, tgt)
		}
	}
	// Targets in b but not in a.
	for tgt := range fromB {
		if _, found := fromA[tgt]; !found {
			onlyB = append(onlyB, tgt)
		}
	}
	return onlyA, onlyB, nil
}

// index assigns an internal ID to a note if it hasn't been seen before.
// TODO: push create event
func (ps *PersistentCache) index(path Path) {
	if _, exists := ps.idx[path]; exists {
		return
	}
	ps.idxCounter++
	ps.idx[path] = ps.idxCounter
}

// moveIndex reassigns an index from a start path to a destination path.
// It assumes that the note identified by start has no backlinks.
func (ps *PersistentCache) moveIndex(start Path, dest Path) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if id, exists := ps.idx[start]; exists {
		delete(ps.idx, start)
		ps.idx[dest] = id
	}
}

// getNoteInfo retrieves note information from the temporary layer first,
// and then falls back to the persistent layer.
func (ps *PersistentCache) getNoteInfo(path Path) (Note, bool) {
	if info, ok := ps.tmpLayer.info(path); ok {
		return info, true
	}
	return ps.pstLayer.info(path)
}

// deindex removes a note from the provided layer if it is missing and has no backlinks.
// TODO: remove note from index
func (ps *PersistentCache) deindex(path Path, cl cacheLayer) error {
	note, ok := cl.info(path)
	if !ok {
		return fmt.Errorf("Note %s does not exist.", path)
	}

	if !note.missing {
		return nil
	}

	backlinks, err := cl.backLinks(path)
	if err != nil {
		return err
	}

	if len(backlinks) > 0 {
		return nil
	}

	_ = cl.delete(path)
	if _, exists := ps.getNoteInfo(path); !exists {
		// TODO: remove from index
		ps.noteDeleteEvent(path)
	}

	return nil
}

// prepareTarget checks if a target exists; if not, it creates a missing note in the persistent layer.
func (ps *PersistentCache) prepareTarget(src Path, link Link, cl cacheLayer) error {
	if link.Src != src {
		return fmt.Errorf("link source does not match with path")
	}

	if link.Tgt == src {
		return nil
	}

	if _, exists := cl.info(link.Tgt); exists {
		return nil
	}

	// Create a missing note in the persistent layer.
	err := ps.pstLayer.upsert(Note{Path: link.Tgt, missing: true}, []Link{})
	if err != nil {
		return fmt.Errorf("failed to upsert missing target note %s: %w", link.Tgt, err)
	}
	ps.index(link.Tgt)
	// Publishing logic is untouched.
	ps.linkCreateEvent(link.Src, link.Tgt)
	return nil
}

func (ps *PersistentCache) linkCreateEvent(src, tgt Path) error {
	return nil
}

func (ps *PersistentCache) linkDeleteEvent(src, tgt Path) error {
	return nil
}

func (ps *PersistentCache) noteCreateEvent(note Note) error {
	return nil
}

func (ps *PersistentCache) noteUpdateEvent(note Note) error {
	return nil
}

func (ps *PersistentCache) noteDeleteEvent(path Path) error {
	return nil
}

// applyUpsert is a helper that applies upsert on a given layer, updates links diff, and triggers events.
// TODO: push note creation event
func (ps *PersistentCache) applyUpsert(note Note, links []Link, cl cacheLayer) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	linksBefore, _ := ps.ForwardLinks(note.Path)

	// Prepare targets for all links.
	for _, link := range links {
		if err := ps.prepareTarget(note.Path, link, cl); err != nil {
			return err
		}
	}

	// Upsert the note in the selected layer.
	cl.upsert(note, links)

	linksAfter, _ := ps.ForwardLinks(note.Path)

	removed, added, err := diff(note.Path, linksBefore, linksAfter)
	if err != nil {
		return err
	}

	for _, tgt := range removed {
		ps.linkDeleteEvent(note.Path, tgt)
		ps.deindex(tgt, cl)
	}
	for _, tgt := range added {
		ps.linkCreateEvent(note.Path, tgt)
	}

	ps.index(note.Path)
	return nil
}

// Upsert inserts or updates a note in the persistent layer.
func (ps *PersistentCache) Upsert(note Note, links []Link) error {
	return ps.applyUpsert(note, links, ps.pstLayer)
}

// UpsertTmp inserts or updates a note in the temporary layer.
func (ps *PersistentCache) UpsertTmp(note Note, links []Link) error {
	return ps.applyUpsert(note, links, ps.tmpLayer)
}

// applyDelete is a helper that handles deletion in a given layer.
// TODO: check that the note exists
func (ps *PersistentCache) applyDelete(path Path, cl cacheLayer) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Remove outgoing links by marking note as missing.
	if err := ps.applyUpsert(Note{Path: path, missing: true}, []Link{}, cl); err != nil {
		return err
	}

	if err := ps.deindex(path, cl); err != nil {
		return err
	}
	return nil
}

// Delete removes a note from the persistent layer.
func (ps *PersistentCache) Delete(path Path) error {
	return ps.applyDelete(path, ps.pstLayer)
}

// DeleteTmp removes a note from the temporary layer.
func (ps *PersistentCache) DeleteTmp(path Path) error {
	return ps.applyDelete(path, ps.tmpLayer)
}

// Paths returns the union of note paths in both persistent and temporary layers.
// The temporary layer takes precedence.
func (ps *PersistentCache) Paths() ([]Path, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	pathsPersistent, err := ps.pstLayer.paths()
	if err != nil {
		return nil, err
	}
	pathsTemporary, err := ps.tmpLayer.paths()
	if err != nil {
		return nil, err
	}
	unique := make(map[Path]struct{})
	for _, p := range pathsPersistent {
		unique[p] = struct{}{}
	}
	for _, p := range pathsTemporary {
		unique[p] = struct{}{}
	}
	var result []Path
	for p := range unique {
		result = append(result, p)
	}
	return result, nil
}

// ForwardLinks returns the links originating from the note at the given path.
// The temporary layer takes precedence if a non-missing note is found there.
func (ps *PersistentCache) ForwardLinks(path Path) ([]Link, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if note, exists := ps.tmpLayer.info(path); exists && !note.missing {
		return ps.tmpLayer.forwardLinks(path)
	}
	return ps.pstLayer.forwardLinks(path)
}

// BackLinks returns the links pointing to the note at the given path.
// It merges backlinks from both layers. For backlinks coming from the persistent layer,
// we check if the source note exists in the temporary layer. If it does, then its pst backlink is
// considered outdated (since tmp overrides pst) and is omitted.
func (ps *PersistentCache) BackLinks(path Path) ([]Link, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// Get backlinks from the temporary layer.
	tmpBLinks, err := ps.tmpLayer.backLinks(path)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve temporary backlinks for %s: %w", path, err)
	}

	// Get backlinks from the persistent layer.
	pstBLinks, err := ps.pstLayer.backLinks(path)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve persistent backlinks for %s: %w", path, err)
	}

	// Filter persistent backlinks: include only those whose source note is not present in tmpLayer.
	var filteredPst []Link
	for _, link := range pstBLinks {
		// TODO: Handle missing notes in tmpLayer if necessary.
		if _, exists := ps.tmpLayer.info(link.Src); !exists {
			filteredPst = append(filteredPst, link)
		}
	}
	return append(tmpBLinks, filteredPst...), nil
}

// Flush is currently not implemented. It returns an error.
func (ps *PersistentCache) Flush() error {
	return fmt.Errorf("Flush not implemented")
}

// Subscribe allows clients to receive cache update events.
// A new buffered channel is created for the subscriber, and an initial state is pushed.
func (ps *PersistentCache) Subscribe() error {
	return fmt.Errorf("Subscribe is not implemented")
}
