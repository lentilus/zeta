package cache

import (
	"fmt"
	"log"
	"sync"
)

// noteID is used for identification in cytoscape only.
type noteID uint

// linkID is used for identification in cytoscape only.
type linkID string

// HybridCache holds the temporary and persistent cache layers,
// as well as index and subscriber data.
type HybridCache struct {
	tmpLayer    cacheLayer
	pstLayer    cacheLayer
	idx         map[Path]noteID
	idxCounter  noteID
	subscribers []chan Event
	subMu       sync.RWMutex
	mu          sync.RWMutex
}

// NewHybridCache initializes and returns a new instance of HybridCache.
func NewHybridCache() *HybridCache {
	pc := &HybridCache{
		tmpLayer:    newMapCacheLayer(),
		pstLayer:    newMapCacheLayer(),
		idx:         make(map[Path]noteID),
		idxCounter:  0,
		subscribers: []chan Event{},
		mu:          sync.RWMutex{},
	}
	return pc
}

// indexIfNeeded assigns an internal ID to a note if it hasn't been seen before.
func (ps *HybridCache) indexIfNeeded(path Path) {
	_, ok := ps.idx[path]
	if ok {
		return
	}
	ps.idxCounter++
	id := ps.idxCounter
	ps.idx[path] = id
	note, _ := ps.getNoteInfo(path)
	ps.dispatch(
		Event{
			Operation: "CREATE",
			Group:     "NOTES",
			ID:        fmt.Sprint(id),
			Data:      []any{path, note.missing},
		},
	)
}

// moveIndex reassigns an index from a start path to a destination path.
// It assumes that the note identified by start has no backlinks.
func (ps *HybridCache) moveIndex(start Path, dest Path) {
	if id, exists := ps.idx[start]; exists {
		delete(ps.idx, start)
		ps.idx[dest] = id
	}
}

// getNoteInfo retrieves note information from the temporary layer first,
// and then falls back to the persistent layer.
func (ps *HybridCache) getNoteInfo(path Path) (note, bool) {
	if info, ok := ps.tmpLayer.info(path); ok {
		return info, true
	}
	return ps.pstLayer.info(path)
}

// deindexIfNeeded removes a note from the provided layer if it is missing and has no backlinks.
func (ps *HybridCache) deindexIfNeeded(path Path, cl cacheLayer) error {
	note, ok := cl.info(path)
	if !ok {
		err := fmt.Errorf("%w: %s", ErrNoteNotFound, path)
		return err
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

	if err := cl.delete(path); err != nil {
		return err
	}

	if _, ok := ps.getNoteInfo(path); !ok {
		id, _ := ps.idx[path]
		ps.dispatch(
			Event{
				Operation: "DELETE",
				Group:     "NOTES",
				ID:        fmt.Sprint(id),
				Data:      []any{},
			})
		delete(ps.idx, path)
	}

	return nil
}

// prepareTarget checks if a target exists; if not, it creates a missing note in cache layer.
func (ps *HybridCache) prepareTarget(src Path, link Link, cl cacheLayer) error {
	if link.Src != src {
		err := fmt.Errorf("%w: link source does not match with path", ErrInvalidLink)
		return err
	}

	if link.Tgt == src {
		return nil
	}

	if _, exists := cl.info(link.Tgt); exists {
		return nil
	}

	// Create a missing note in the persistent layer.
	err := cl.upsert(note{Path: link.Tgt, missing: true}, []Link{})
	if err != nil {
		err = fmt.Errorf("failed to upsert missing target note %s: %w", link.Tgt, err)
		return err
	}
	ps.indexIfNeeded(link.Tgt)
	return nil
}

// applyUpsert is a helper that applies upsert on a given layer, updates links diff, and triggers events.
func (ps *HybridCache) applyUpsert(note note, links []Link, cl cacheLayer) error {
	linksBefore, _ := ps.forwardLinks(note.Path)

	// Prepare targets for all links.
	for _, link := range links {
		if err := ps.prepareTarget(note.Path, link, cl); err != nil {
			return err
		}
	}

	// Upsert the note in the selected layer.
	if err := cl.upsert(note, links); err != nil {
		return err
	}
	ps.indexIfNeeded(note.Path)

	linksAfter, _ := ps.forwardLinks(note.Path)

	removed, added, err := diff(note.Path, linksBefore, linksAfter)
	if err != nil {
		return err
	}

	for _, tgt := range removed {
		ps.dispatch(
			Event{
				Operation: "DELETE",
				Group:     "LINKS",
				ID:        fmt.Sprintf("%d-%d", note.Path, tgt),
				Data:      []any{},
			})
		if err := ps.deindexIfNeeded(tgt, cl); err != nil {
			return err
		}
	}

	for _, tgt := range added {
		ps.dispatch(
			Event{
				Operation: "CREATE",
				Group:     "LINKS",
				ID:        fmt.Sprintf("%d-%d", note.Path, tgt),
				Data:      []any{},
			})
	}

	return nil
}

// Upsert inserts or updates a note in the persistent layer.
func (ps *HybridCache) Upsert(path Path, links []Link) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	note := note{Path: path, missing: false}
	return ps.applyUpsert(note, links, ps.pstLayer)
}

// UpsertTmp inserts or updates a note in the temporary layer.
func (ps *HybridCache) UpsertTmp(path Path, links []Link) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	note := note{Path: path, missing: false}
	return ps.applyUpsert(note, links, ps.tmpLayer)
}

// applyDelete is a helper that handles deletion in a given layer.
func (ps *HybridCache) applyDelete(path Path, cl cacheLayer) error {
	if _, ok := cl.info(path); !ok {
		err := fmt.Errorf("%w: %s", ErrNoteNotFound, path)
		return err
	}

	// Remove outgoing links by marking note as missing.
	if err := ps.applyUpsert(note{Path: path, missing: true}, []Link{}, cl); err != nil {
		return err
	}

	if err := ps.deindexIfNeeded(path, cl); err != nil {
		return err
	}
	return nil
}

// Delete removes a note from the persistent layer.
func (ps *HybridCache) Delete(path Path) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.applyDelete(path, ps.pstLayer)
}

// DeleteTmp removes a note from the temporary layer.
func (ps *HybridCache) DeleteTmp(path Path) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.applyDelete(path, ps.tmpLayer)
}

// Flush is currently not implemented. It returns an error.
func (ps *HybridCache) Flush() error {
	return fmt.Errorf("Flush not implemented.")
}

func (ps *HybridCache) dispatch(e Event) error {
	// Validate Event
	if op := e.Operation; op != "CREATE" && op != "UPDATE" && op != "DELETE" {
		return fmt.Errorf("Invalid event operation: %s", op)
	}
	if gp := e.Group; gp != "NOTES" && gp != "LINKS" {
		return fmt.Errorf("Invalid group: %s", gp)
	}

	log.Printf("Sending event: %v", e)

	ps.subMu.RLock()
	defer ps.subMu.RUnlock()
	for _, ch := range ps.subscribers {
		select {
		case ch <- e:
			// Delivered event
		default:
			// TODO: handle full channel
		}
	}
	return nil
}

// Subscribe allows clients to receive cache update events.
// A new buffered channel is created for the subscriber, and an initial state is pushed.
func (ps *HybridCache) Subscribe() error {
	return fmt.Errorf("Subscribe not implemented.")
}
