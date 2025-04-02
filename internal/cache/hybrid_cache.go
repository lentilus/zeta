package cache

import (
	"fmt"
	"log"
	"sync"
)

// Exported error types for use in exported functions.

// noteID is used for identification in cytoscape only.
type noteID uint

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

// NewPersistentCache initializes and returns a new instance of HybridCache.
func NewHybridCache() *HybridCache {
	pc := &HybridCache{
		tmpLayer:    newMapCacheLayer(),
		pstLayer:    newMapCacheLayer(),
		idx:         make(map[Path]noteID),
		idxCounter:  0,
		subscribers: []chan Event{},
		mu:          sync.RWMutex{},
	}
	log.Println("PersistentCache created")
	return pc
}

func (ps *HybridCache) sprint() string {
	template := `-----
    index: %s
    MERGED
    - notes: %s
    TMP
    - notes: %s
    PST:
    - notes: %s
    -----`
	return fmt.Sprintf(
		template,
		fmt.Sprint(ps.idx),
		fmt.Sprint(ps.Paths()),
		fmt.Sprint(ps.tmpLayer.paths()),
		fmt.Sprint(ps.pstLayer.paths()),
	)
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
			log.Println("getTargets error:", err)
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

// indexIfNeeded assigns an internal ID to a note if it hasn't been seen before.
func (ps *HybridCache) indexIfNeeded(path Path) {
	if _, exists := ps.idx[path]; exists {
		return
	}
	ps.idxCounter++
	ps.idx[path] = ps.idxCounter
	ps.noteCreateEvent(path)
	log.Printf("Indexed note %s with id %d", path, ps.idxCounter)
}

// moveIndex reassigns an index from a start path to a destination path.
// It assumes that the note identified by start has no backlinks.
func (ps *HybridCache) moveIndex(start Path, dest Path) {
	if id, exists := ps.idx[start]; exists {
		delete(ps.idx, start)
		ps.idx[dest] = id
		log.Printf("Moved index from %s to %s", start, dest)
	}
}

// getNoteInfo retrieves note information from the temporary layer first,
// and then falls back to the persistent layer.
func (ps *HybridCache) getNoteInfo(path Path) (Note, bool) {
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
		log.Println("deindexIfNeeded error:", err)
		return err
	}

	if !note.missing {
		return nil
	}

	backlinks, err := cl.backLinks(path)
	if err != nil {
		log.Println("deindexIfNeeded backlinks error:", err)
		return err
	}

	if len(backlinks) > 0 {
		return nil
	}

	_ = cl.delete(path)
	if _, exists := ps.getNoteInfo(path); !exists {
		delete(ps.idx, path)
		ps.noteDeleteEvent(path)
		log.Printf("De-indexed note %s", path)
	}

	return nil
}

// prepareTarget checks if a target exists; if not, it creates a missing note in the persistent layer.
func (ps *HybridCache) prepareTarget(src Path, link Link, cl cacheLayer) error {
	if link.Src != src {
		err := fmt.Errorf("%w: link source does not match with path", ErrInvalidLink)
		log.Println("prepareTarget error:", err)
		return err
	}

	if link.Tgt == src {
		return nil
	}

	if _, exists := cl.info(link.Tgt); exists {
		return nil
	}

	// Create a missing note in the persistent layer.
	err := cl.upsert(Note{Path: link.Tgt, missing: true}, []Link{})
	if err != nil {
		err = fmt.Errorf("failed to upsert missing target note %s: %w", link.Tgt, err)
		log.Println("prepareTarget error:", err)
		return err
	}
	ps.indexIfNeeded(link.Tgt)
	ps.linkCreateEvent(link.Src, link.Tgt)
	log.Printf("Prepared target %s for source %s", link.Tgt, src)
	return nil
}

func (ps *HybridCache) linkCreateEvent(src, tgt Path) error {
	// Logging the event.
	log.Printf("Link created from %s to %s", src, tgt)
	return nil
}

func (ps *HybridCache) linkDeleteEvent(src, tgt Path) error {
	log.Printf("Link deleted from %s to %s", src, tgt)
	return nil
}

func (ps *HybridCache) noteCreateEvent(path Path) error {
	log.Printf("Note created: %s", path)
	return nil
}

func (ps *HybridCache) noteUpdateEvent(note Note) error {
	log.Printf("Note updated: %s", note.Path)
	return nil
}

func (ps *HybridCache) noteDeleteEvent(path Path) error {
	log.Printf("Note deleted: %s", path)
	return nil
}

// applyUpsert is a helper that applies upsert on a given layer, updates links diff, and triggers events.
func (ps *HybridCache) applyUpsert(note Note, links []Link, cl cacheLayer) error {
	linksBefore, _ := ps.forwardLinks(note.Path)

	// Prepare targets for all links.
	for _, link := range links {
		if err := ps.prepareTarget(note.Path, link, cl); err != nil {
			return err
		}
	}

	// Upsert the note in the selected layer.
	if err := cl.upsert(note, links); err != nil {
		log.Printf("applyUpsert error on upsert for note %s: %v", note.Path, err)
		return err
	}

	linksAfter, _ := ps.forwardLinks(note.Path)

	removed, added, err := diff(note.Path, linksBefore, linksAfter)
	if err != nil {
		log.Printf("applyUpsert diff error for note %s: %v", note.Path, err)
		return err
	}

	for _, tgt := range removed {
		ps.linkDeleteEvent(note.Path, tgt)
		if err := ps.deindexIfNeeded(tgt, cl); err != nil {
			log.Printf("applyUpsert deindex error for target %s: %v", tgt, err)
			return err
		}
	}
	for _, tgt := range added {
		ps.linkCreateEvent(note.Path, tgt)
	}

	ps.indexIfNeeded(note.Path)
	log.Printf("Upsert applied for note %s", note.Path)
	return nil
}

// Upsert inserts or updates a note in the persistent layer.
func (ps *HybridCache) Upsert(note Note, links []Link) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	log.Printf("Upsert called for note %s", note.Path)
	return ps.applyUpsert(note, links, ps.pstLayer)
}

// UpsertTmp inserts or updates a note in the temporary layer.
func (ps *HybridCache) UpsertTmp(note Note, links []Link) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	log.Printf("UpsertTmp called for note %s", note.Path)
	return ps.applyUpsert(note, links, ps.tmpLayer)
}

// applyDelete is a helper that handles deletion in a given layer.
func (ps *HybridCache) applyDelete(path Path, cl cacheLayer) error {
	if _, ok := cl.info(path); !ok {
		err := fmt.Errorf("%w: %s", ErrNoteNotFound, path)
		log.Println("applyDelete error:", err)
		return err
	}

	// Remove outgoing links by marking note as missing.
	if err := ps.applyUpsert(Note{Path: path, missing: true}, []Link{}, cl); err != nil {
		log.Printf("applyDelete upsert error for note %s: %v", path, err)
		return err
	}

	if err := ps.deindexIfNeeded(path, cl); err != nil {
		log.Printf("applyDelete deindex error for note %s: %v", path, err)
		return err
	}
	log.Printf("Deleted note %s", path)
	return nil
}

// Delete removes a note from the persistent layer.
func (ps *HybridCache) Delete(path Path) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	log.Printf("Delete called for note %s", path)
	return ps.applyDelete(path, ps.pstLayer)
}

// DeleteTmp removes a note from the temporary layer.
func (ps *HybridCache) DeleteTmp(path Path) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	log.Printf("DeleteTmp called for note %s", path)
	return ps.applyDelete(path, ps.tmpLayer)
}

// Paths returns the union of note paths in both persistent and temporary layers.
// The temporary layer takes precedence.
func (ps *HybridCache) Paths() ([]Path, error) {
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

func (ps *HybridCache) forwardLinks(path Path) ([]Link, error) {
	if note, exists := ps.tmpLayer.info(path); exists && !note.missing {
		return ps.tmpLayer.forwardLinks(path)
	}
	return ps.pstLayer.forwardLinks(path)
}

// ForwardLinks returns the links originating from the note at the given path.
// The temporary layer takes precedence if a non-missing note is found there.
func (ps *HybridCache) ForwardLinks(path Path) ([]Link, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.forwardLinks(path)
}

// BackLinks returns the links pointing to the note at the given path.
// It merges backlinks from both layers. For backlinks coming from the persistent layer,
// we check if the source note exists in the temporary layer. If it does, then its persistent backlink is
// considered outdated (since tmp overrides pst) and is omitted.
func (ps *HybridCache) backLinks(path Path) ([]Link, error) {
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

func (ps *HybridCache) BackLinks(path Path) ([]Link, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.backLinks(path)
}

// Flush is currently not implemented. It returns an error.
func (ps *HybridCache) Flush() error {
	log.Println("Flush called but not implemented")
	return fmt.Errorf("Flush not implemented.")
}

// Subscribe allows clients to receive cache update events.
// A new buffered channel is created for the subscriber, and an initial state is pushed.
func (ps *HybridCache) Subscribe() error {
	log.Println("Subscribe called but not implemented")
	return fmt.Errorf("Subscribe not implemented.")
}
