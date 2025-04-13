package cache

import (
	"fmt"
	"log"
	"sync"
)

// noteID is used for identification.
type noteID uint

// linkID is used for identification.
type linkID string

// HybridCache holds temporary and persistent cache layers, an index, and subscriber data.
type HybridCache struct {
	tmpLayer    cacheLayer
	pstLayer    cacheLayer
	idx         map[Path]noteID
	idxCounter  noteID
	subscribers []chan Event
	subMu       sync.RWMutex
	mu          sync.RWMutex
}

// NewHybridCache creates a new HybridCache instance.
func NewHybridCache() *HybridCache {
	return &HybridCache{
		tmpLayer:    newMapCacheLayer(),
		pstLayer:    newMapCacheLayer(),
		idx:         make(map[Path]noteID),
		idxCounter:  0,
		subscribers: []chan Event{},
	}
}

// assignIndexIfNeeded assigns an ID to a note if it isn't indexed yet.
func (cache *HybridCache) assignIndexIfNeeded(path Path) {
	if _, exists := cache.idx[path]; exists {
		return
	}
	cache.idxCounter++
	newID := cache.idxCounter
	cache.idx[path] = newID

	noteInfo, _ := cache.getNoteInfo(path)
	cache.sendEvent(Event{
		Operation: "CREATE",
		Group:     "NOTES",
		ID:        fmt.Sprint(newID),
		Data:      []any{path, noteInfo.missing},
	})
}

// moveIndex transfers the index from one note to another.
func (cache *HybridCache) moveIndex(start, dest Path) {
	if id, exists := cache.idx[start]; exists {
		delete(cache.idx, start)
		cache.idx[dest] = id
	}
}

// getNoteInfo retrieves note information from the temporary layer first, then persistent layer.
func (cache *HybridCache) getNoteInfo(path Path) (note, bool) {
	if info, ok := cache.tmpLayer.info(path); ok {
		return info, true
	}
	return cache.pstLayer.info(path)
}

// removeIndexIfOrphan removes the note from the index and from the layer if it's missing and has no backlinks.
func (cache *HybridCache) removeIndexIfOrphan(path Path, layer cacheLayer) error {
	n, exists := layer.info(path)
	if !exists {
		return fmt.Errorf("%w: %s", ErrNoteNotFound, path)
	}

	if !n.missing {
		return nil
	}

	backlinks, err := layer.backLinks(path)
	if err != nil {
		return err
	}
	if len(backlinks) > 0 {
		return nil
	}

	if err := layer.delete(path); err != nil {
		return err
	}

	if _, exists := cache.getNoteInfo(path); !exists {
		if id, ok := cache.idx[path]; ok {
			cache.sendEvent(Event{
				Operation: "DELETE",
				Group:     "NOTES",
				ID:        fmt.Sprint(id),
				Data:      []any{},
			})
			delete(cache.idx, path)
		}
	}

	return nil
}

// ensureTargetExists verifies if a linked target exists or creates a missing note if not.
func (cache *HybridCache) ensureTargetExists(src Path, link Link, layer cacheLayer) error {
	if link.Src != src {
		return fmt.Errorf("%w: link source does not match with path", ErrInvalidLink)
	}
	if link.Tgt == src {
		return nil
	}
	if _, exists := layer.info(link.Tgt); exists {
		return nil
	}
	// Create a missing note in the layer.
	if err := layer.upsert(note{Path: link.Tgt, missing: true}, []Link{}); err != nil {
		return fmt.Errorf("failed to upsert missing target note %s: %w", link.Tgt, err)
	}
	cache.assignIndexIfNeeded(link.Tgt)
	return nil
}

// prepareAllTargets loops over the links and ensures each target exists.
func (cache *HybridCache) prepareAllTargets(src Path, links []Link, layer cacheLayer) error {
	for _, link := range links {
		if err := cache.ensureTargetExists(src, link, layer); err != nil {
			return err
		}
	}
	return nil
}

// updateNoteInLayer upserts the note into the layer and updates its index.
func (cache *HybridCache) updateNoteInLayer(n note, links []Link, layer cacheLayer) error {
	if err := layer.upsert(n, links); err != nil {
		return err
	}
	cache.assignIndexIfNeeded(n.Path)
	return nil
}

// computeDiffAndDispatchEvents computes the diff between pre-upsert and post-upsert link states and dispatches events.
func (cache *HybridCache) computeDiffAndDispatchEvents(
	notePath Path,
	prevLinks, newLinks []Link,
	layer cacheLayer,
) error {
	removed, added, err := diff(notePath, prevLinks, newLinks)
	if err != nil {
		return err
	}

	// Dispatch events for removed links.
	for _, tgt := range removed {
		cache.sendEvent(Event{
			Operation: "DELETE",
			Group:     "LINKS",
			ID:        fmt.Sprintf("%d-%d", notePath, tgt),
			Data:      []any{},
		})
		if err := cache.removeIndexIfOrphan(tgt, layer); err != nil {
			return err
		}
	}
	// Dispatch events for added links.
	for _, tgt := range added {
		cache.sendEvent(Event{
			Operation: "CREATE",
			Group:     "LINKS",
			ID:        fmt.Sprintf("%d-%d", notePath, tgt),
			Data:      []any{},
		})
	}
	return nil
}

// applyUpsert performs the full upsert workflow and computes the diff correctly.
func (cache *HybridCache) applyUpsert(n note, links []Link, layer cacheLayer) error {
	// Prepare targets for each link.
	if err := cache.prepareAllTargets(n.Path, links, layer); err != nil {
		return err
	}

	// Capture links state before the upsert.
	prevLinks, _ := cache.forwardLinks(n.Path)

	// Update the note in the layer.
	if err := cache.updateNoteInLayer(n, links, layer); err != nil {
		return err
	}

	// Capture links state after the upsert.
	newLinks, _ := cache.forwardLinks(n.Path)

	// Compute diff and dispatch appropriate events.
	if err := cache.computeDiffAndDispatchEvents(n.Path, prevLinks, newLinks, layer); err != nil {
		return err
	}
	return nil
}

// Upsert inserts or updates a note in the persistent layer.
func (cache *HybridCache) Upsert(path Path, links []Link) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	n := note{Path: path, missing: false}
	return cache.applyUpsert(n, links, cache.pstLayer)
}

// UpsertTmp inserts or updates a note in the temporary layer.
func (cache *HybridCache) UpsertTmp(path Path, links []Link) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	n := note{Path: path, missing: false}
	return cache.applyUpsert(n, links, cache.tmpLayer)
}

// applyDelete performs deletion operations on a note from the specified layer.
func (cache *HybridCache) applyDelete(path Path, layer cacheLayer) error {
	if _, exists := layer.info(path); !exists {
		return fmt.Errorf("%w: %s", ErrNoteNotFound, path)
	}

	// Mark the note as missing and remove outgoing links.
	if err := cache.applyUpsert(note{Path: path, missing: true}, []Link{}, layer); err != nil {
		return err
	}

	return cache.removeIndexIfOrphan(path, layer)
}

// Delete removes a note from the persistent layer.
func (cache *HybridCache) Delete(path Path) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.applyDelete(path, cache.pstLayer)
}

// DeleteTmp removes a note from the temporary layer.
func (cache *HybridCache) DeleteTmp(path Path) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.applyDelete(path, cache.tmpLayer)
}

// Flush returns an error as the operation is not implemented.
func (cache *HybridCache) Flush() error {
	return fmt.Errorf("Flush not implemented.")
}

// sendEvent validates and sends an event to all subscribers.
func (cache *HybridCache) sendEvent(e Event) error {
	if e.Operation != "CREATE" && e.Operation != "UPDATE" && e.Operation != "DELETE" {
		return fmt.Errorf("Invalid event operation: %s", e.Operation)
	}
	if e.Group != "NOTES" && e.Group != "LINKS" {
		return fmt.Errorf("Invalid group: %s", e.Group)
	}

	log.Printf("Sending event: %v", e)

	cache.subMu.RLock()
	defer cache.subMu.RUnlock()
	for _, ch := range cache.subscribers {
		select {
		case ch <- e:
		default:
			// Optionally handle full channel case.
		}
	}
	return nil
}

// Subscribe is not implemented.
func (cache *HybridCache) Subscribe() error {
	return fmt.Errorf("Subscribe not implemented.")
}
