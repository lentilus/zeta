package cache

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// noteID is used for identification.
type noteID uint

// linkID is used for identification.
type linkID string

// Hybrid holds temporary and persistent cache layers, an index, and subscriber data.
type Hybrid struct {
	tmpLayer    layer
	pstLayer    layer
	idx         map[Path]noteID
	idxCounter  noteID
	subscribers []chan Event
	subMu       sync.RWMutex
	mu          sync.RWMutex
}

// NewHybrid creates a new Hybrid cache instance.
func NewHybrid() *Hybrid {
	return &Hybrid{
		tmpLayer:    newHashmapLayer(),
		pstLayer:    newHashmapLayer(),
		idx:         make(map[Path]noteID),
		idxCounter:  0,
		subscribers: []chan Event{},
	}
}

// assignIndexIfNeeded assigns an ID to a note if it isn't indexed yet.
func (cache *Hybrid) assignIndexIfNeeded(path Path) {
	if _, exists := cache.idx[path]; exists {
		return
	}
	cache.idxCounter++
	newID := cache.idxCounter
	cache.idx[path] = newID

	noteInfo, _ := cache.getNoteInfo(path)
	cache.sendEvent(Event{
		Operation: "createNote",
		Note: NoteData{
			ID:      int(newID),
			Path:    string(path),
			Missing: noteInfo.missing,
		},
	})
}

// moveIndex transfers the index from one note to another.
func (cache *Hybrid) moveIndex(start, dest Path) {
	id, exists := cache.idx[start]
	if !exists {
		panic("Does not exist")
	}
	delete(cache.idx, start)
	cache.idx[dest] = id

	cache.sendEvent(Event{
		Operation: "updateNote",
		Note: NoteData{
			ID:      int(id),
			Path:    string(dest),
			Missing: true, // is always true
		},
	})
}

// getNoteInfo retrieves note information from the temporary layer first, then persistent layer.
func (cache *Hybrid) getNoteInfo(path Path) (note, bool) {
	if info, ok := cache.tmpLayer.info(path); ok {
		return info, true
	}
	return cache.pstLayer.info(path)
}

// removeIndexIfOrphan removes the note from the index and from the layer if it's missing and has no backlinks.
func (cache *Hybrid) removeIndexIfOrphan(path Path, layer layer) error {
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
				Operation: "deleteNote",
				Note: NoteData{
					ID: int(id),
				},
			})
			delete(cache.idx, path)
		}
	}

	return nil
}

// ensureTargetExists verifies if a linked target exists or creates a missing note if not.
func (cache *Hybrid) ensureTargetExists(src Path, link Link, layer layer) error {
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
func (cache *Hybrid) prepareAllTargets(src Path, links []Link, layer layer) error {
	for _, link := range links {
		if err := cache.ensureTargetExists(src, link, layer); err != nil {
			return err
		}
	}
	return nil
}

// updateNoteInLayer upserts the note into the layer and updates its index.
func (cache *Hybrid) updateNoteInLayer(n note, links []Link, layer layer) error {
	if err := layer.upsert(n, links); err != nil {
		return err
	}
	cache.assignIndexIfNeeded(n.Path)
	return nil
}

// computeDiffAndDispatchEvents computes the diff between pre-upsert and post-upsert link states and dispatches events.
func (cache *Hybrid) computeDiffAndDispatchEvents(
	notePath Path,
	prevLinks, newLinks []Link,
	layer layer,
	sendEvents bool,
) error {
	removed, added, err := diff(notePath, prevLinks, newLinks)
	if err != nil {
		return err
	}

	// Dispatch events for removed links.
	for _, tgt := range removed {
		srcID, _ := cache.idx[notePath]
		tgtID, _ := cache.idx[tgt]
		if err := cache.removeIndexIfOrphan(tgt, layer); err != nil {
			return err
		}
		if !sendEvents {
			continue
		}
		cache.sendEvent(Event{
			Operation: "deleteLink",
			Link: LinkData{
				SourceID: int(srcID),
				TargetID: int(tgtID),
			},
		})
	}
	// Dispatch events for added links.
	for _, tgt := range added {
		if !sendEvents {
			continue
		}
		cache.sendEvent(Event{
			Operation: "createLink",
			Link: LinkData{
				SourceID: int(cache.idx[notePath]),
				TargetID: int(cache.idx[tgt]),
			},
		})
	}
	return nil
}

// applyUpsert performs the full upsert workflow and computes the diff correctly.
func (cache *Hybrid) applyUpsert(n note, links []Link, layer layer) error {
	// Capture links state before the upsert.
	prevLinks, _ := cache.forwardLinks(n.Path)

	// Do special update if possible.
	didSpecialUpdate := false
	if layer == cache.tmpLayer {
		didSpecialUpdate = cache.specialUpdate(n.Path, prevLinks, links)
	}

	// Prepare targets for each link.
	if err := cache.prepareAllTargets(n.Path, links, layer); err != nil {
		return err
	}

	// Update the note in the layer.
	if err := cache.updateNoteInLayer(n, links, layer); err != nil {
		return err
	}

	// Capture links state after the upsert.
	newLinks, _ := cache.forwardLinks(n.Path)

	// Compute diff and dispatch appropriate events. Skip events if specialUpdate occured.
	if err := cache.computeDiffAndDispatchEvents(n.Path, prevLinks, newLinks, layer, !didSpecialUpdate); err != nil {
		return err
	}

	return nil
}

// Upsert inserts or updates a note in the persistent layer.
func (cache *Hybrid) Upsert(path Path, links []Link) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	n := note{Path: path, missing: false}
	return cache.applyUpsert(n, links, cache.pstLayer)
}

// UpsertTmp inserts or updates a note in the temporary layer.
func (cache *Hybrid) UpsertTmp(path Path, links []Link) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	n := note{Path: path, missing: false}
	return cache.applyUpsert(n, links, cache.tmpLayer)
}

// applyDelete performs deletion operations on a note from the specified layer.
func (cache *Hybrid) applyDelete(path Path, layer layer) error {
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
func (cache *Hybrid) Delete(path Path) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.applyDelete(path, cache.pstLayer)
}

// DeleteTmp removes a note from the temporary layer.
func (cache *Hybrid) DeleteTmp(path Path) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.applyDelete(path, cache.tmpLayer)
}

// Paths returns the union of note paths in both persistent and temporary layers.
// The temporary layer takes precedence.
func (ps *Hybrid) Paths() ([]Path, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	pathsPersistent, err := ps.pstLayer.paths()
	if err != nil {
		log.Printf("Paths error retrieving persistent layer paths: %v", err)
		return nil, err
	}
	pathsTemporary, err := ps.tmpLayer.paths()
	if err != nil {
		log.Printf("Paths error retrieving temporary layer paths: %v", err)
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

func (ps *Hybrid) forwardLinks(path Path) ([]Link, error) {
	if note, exists := ps.tmpLayer.info(path); exists && !note.missing {
		return ps.tmpLayer.forwardLinks(path)
	}
	return ps.pstLayer.forwardLinks(path)
}

// ForwardLinks returns the links originating from the note at the given path.
// The temporary layer takes precedence if a non-missing note is found there.
func (ps *Hybrid) ForwardLinks(path Path) ([]Link, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.forwardLinks(path)
}

// BackLinks returns the links pointing to the note at the given path.
// It merges backlinks from both layers. For backlinks coming from the persistent layer,
// we check if the source note exists in the temporary layer. If it does, then its persistent backlink is
// considered outdated (since tmp overrides pst) and is omitted.
func (ps *Hybrid) backLinks(path Path) ([]Link, error) {
	tmpBLinks, err := ps.tmpLayer.backLinks(path)
	if err != nil {
		err = fmt.Errorf("failed to retrieve temporary backlinks for %s: %w", path, err)
		log.Println("backLinks error:", err)
		return nil, err
	}

	pstBLinks, err := ps.pstLayer.backLinks(path)
	if err != nil {
		err = fmt.Errorf("failed to retrieve persistent backlinks for %s: %w", path, err)
		log.Println("backLinks error:", err)
		return nil, err
	}

	var filteredPst []Link
	for _, link := range pstBLinks {
		if info, exists := ps.tmpLayer.info(link.Src); !exists || info.missing {
			filteredPst = append(filteredPst, link)
		}
	}
	return append(tmpBLinks, filteredPst...), nil
}

func (ps *Hybrid) BackLinks(path Path) ([]Link, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.backLinks(path)
}

func (cache *Hybrid) Timestamp(path Path) (time.Time, error) {
	note, ok := cache.pstLayer.info(path)
	if !ok {
		return time.Now(), ErrNoteNotFound
	}
	return note.Timestamp, nil
}

// Flush returns an error as the operation is not implemented.
func (cache *Hybrid) Flush() error {
	return fmt.Errorf("Flush not implemented.")
}

// sendEvent validates and sends an event to all subscribers.
func (cache *Hybrid) sendEvent(e Event) error {
	log.Printf("Sending event: %v", e)

	cache.subMu.RLock()
	defer cache.subMu.RUnlock()
	for _, ch := range cache.subscribers {
		ch <- e // NOTE: May block, but wont drop events
	}
	return nil
}

// Subscribe returns a read-only event channel and a close function which, when called,
// unsubscribes the channel from the cache. This allows the subscriber to both receive all
// future events and to cleanly disconnect when desired.
func (cache *Hybrid) Subscribe() (<-chan Event, func(), error) {
	// Create a buffered channel so that occasional slow consumers do not block event dispatch.
	ch := make(chan Event, 100)

	// Add the new subscriber channel to the cache's subscribers list.
	cache.subMu.Lock()
	cache.subscribers = append(cache.subscribers, ch)
	cache.subMu.Unlock()

	// Create a close function that will remove the channel from the subscribers list and close it.
	closeFn := func() {
		cache.subMu.Lock()
		defer cache.subMu.Unlock()
		for i, sub := range cache.subscribers {
			if sub == ch {
				// Remove the subscriber from the slice.
				cache.subscribers = append(cache.subscribers[:i], cache.subscribers[i+1:]...)
				break
			}
		}
		close(ch)
	}

	// Launch a goroutine to send the initial state (all current notes and links) to the new subscriber.
	go func() {
		cache.mu.RLock()
		defer cache.mu.RUnlock()

		// Get the current note paths.
		paths, err := cache.Paths()
		if err != nil {
			log.Printf("Error getting paths for subscription: %v", err)
			return
		}

		// For each note, create a CREATE event.
		for _, p := range paths {
			n, exists := cache.getNoteInfo(p)
			if !exists {
				continue
			}

			// Retrieve the note index.
			cache.subMu.RLock()
			id, idExists := cache.idx[p]
			cache.subMu.RUnlock()
			if !idExists {
				// Skip if the note is not indexed.
				continue
			}

			event := Event{
				Operation: "createNote",
				Note: NoteData{
					ID:      int(id),
					Path:    string(p),
					Missing: n.missing,
				},
			}
			// Send the note event to the subscriber.
			ch <- event
		}

		// For each note, push events for all outgoing links.
		for _, p := range paths {
			links, err := cache.ForwardLinks(p)
			if err != nil {
				log.Printf("Error getting forward links for %s: %v", p, err)
				continue
			}
			for _, l := range links {
				event := Event{
					Operation: "createLink",
					Link: LinkData{
						SourceID: int(cache.idx[p]),
						TargetID: int(cache.idx[l.Tgt]),
					},
				}
				// Send the link event to the subscriber.
				ch <- event
			}
		}
	}()

	return ch, closeFn, nil
}

func (cache *Hybrid) specialUpdate(path Path, oldLinks, newLinks []Link) bool {
	willremove, willadd, err := diff(path, oldLinks, newLinks)
	if err != nil {
		return false
	}

	if len(willadd) != 1 || len(willremove) != 1 {
		return false
	}

	add := willadd[0]
	rem := willremove[0]

	if info, notMissing := cache.getNoteInfo(add); notMissing {
		log.Printf("---> Note %s not missing: %s", add, info)
		return false
	}

	// Check that this note is the only one that links to `rem`
	blinks, _ := cache.backLinks(rem)
	if len(blinks) != 1 {
		return false
	}

	log.Println("---DID REINDEX---")
	cache.moveIndex(rem, add)
	// err = cache.tmpLayer.delete(rem)
	// log.Println(err)

	log.Printf("-->Index is currently: %s", fmt.Sprint(cache.idx))

	return true
}

// getTargets builds a map from target paths for the given links,
// ensuring that all links have the same source path.
func getTargets(path Path, links []Link) (map[Path]struct{}, error) {
	m := make(map[Path]struct{}, len(links))
	for _, link := range links {
		if link.Src != path {
			err := fmt.Errorf(
				"%w: src %s does not match note path %s",
				ErrInvalidLink,
				link.Src,
				path,
			)
			return nil, err
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
