package cache

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// noteID is used for identification.
type noteID uint

// linkID is used for identification.
type linkID string

// dumpNote is the JSON‐serializable form of a single note.
type dumpNote struct {
	Path      Path      `json:"path"`
	Missing   bool      `json:"missing"`
	Timestamp time.Time `json:"timestamp"`
	Links     []Link    `json:"links"`
}

// dumpPayload is the overall structure for dump/restore.
type dumpPayload struct {
	PstLayer []dumpNote      `json:"pst_layer"`
	Idx      map[Path]noteID `json:"idx"`
	Counter  noteID          `json:"counter"`
}

// Hybrid holds two layers (temporary and persistent), an index, and subscribers for events.
type Hybrid struct {
	tmp     layer
	pst     layer
	idx     map[Path]noteID
	counter noteID
	subs    map[chan Event]struct{}
	mu      sync.RWMutex
	evtMu   sync.RWMutex
}

// NewHybrid initializes an empty Hybrid cache.
func NewHybrid() *Hybrid {
	return &Hybrid{
		tmp:     newHashmapLayer(),
		pst:     newHashmapLayer(),
		idx:     make(map[Path]noteID),
		subs:    make(map[chan Event]struct{}),
		counter: 0,
	}
}

// Dump serializes only the persistent layer + index into JSON.
// The tmp layer is intentionally ignored.
func (h *Hybrid) Dump() ([]byte, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// collect dumpNotes from persistent layer
	paths, err := h.pst.paths()
	if err != nil {
		return nil, fmt.Errorf("dump: failed to list paths: %w", err)
	}
	notes := make([]dumpNote, 0, len(paths))
	for _, p := range paths {
		info, exists := h.pst.info(p)
		if !exists {
			return nil, fmt.Errorf("dump: info for path not found: %s", p)
		}
		links, err := h.pst.forwardLinks(p)
		if err != nil {
			return nil, fmt.Errorf("dump: failed forwardLinks for %s: %w", p, err)
		}
		notes = append(notes, dumpNote{
			Path:      p,
			Missing:   info.missing,
			Timestamp: info.Timestamp,
			Links:     links,
		})
	}

	payload := dumpPayload{
		PstLayer: notes,
		Idx:      h.idx,
		Counter:  h.counter,
	}
	return json.MarshalIndent(payload, "", "  ")
}

// Restore rebuilds a Hybrid from JSON, repopulating only the persistent layer.
// tmp layer is initialized empty.
func Restore(data []byte) (*Hybrid, error) {
	var payload dumpPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache dump: %w", err)
	}

	h := &Hybrid{
		tmp:     newHashmapLayer(),
		pst:     newHashmapLayer(),
		idx:     payload.Idx,
		subs:    make(map[chan Event]struct{}),
		counter: payload.Counter,
	}

	// two-pass restore to ensure links reference existing notes
	for _, dn := range payload.PstLayer {
		n := note{Path: dn.Path, missing: dn.Missing, Timestamp: dn.Timestamp}
		if err := h.pst.upsert(n, nil); err != nil {
			return nil, fmt.Errorf("restore pass 1 %s: %w", dn.Path, err)
		}
	}
	for _, dn := range payload.PstLayer {
		n := note{Path: dn.Path, missing: dn.Missing, Timestamp: dn.Timestamp}
		if err := h.pst.upsert(n, dn.Links); err != nil {
			return nil, fmt.Errorf("restore pass 2 %s: %w", dn.Path, err)
		}
	}

	return h, nil
}

// Upsert inserts or updates a note in the persistent layer.
func (h *Hybrid) Upsert(path Path, links []Link, ts time.Time) error {
	return h.apply(h.pst, note{Path: path, Timestamp: ts, missing: false}, links)
}

// UpsertTmp inserts or updates a note in the temporary layer.
func (h *Hybrid) UpsertTmp(path Path, links []Link) error {
	return h.apply(h.tmp, note{Path: path, missing: false}, links)
}

// Delete removes a note from the persistent layer.
func (h *Hybrid) Delete(path Path) error {
	return h.delete(h.pst, path)
}

// DeleteTmp removes a note from the temporary layer.
func (h *Hybrid) DeleteTmp(path Path) error {
	return h.delete(h.tmp, path)
}

// Subscribe adds a subscriber channel and returns a close function.
func (h *Hybrid) Subscribe() (<-chan Event, func(), error) {
	ch := make(chan Event, 100)
	h.evtMu.Lock()
	h.subs[ch] = struct{}{}
	h.evtMu.Unlock()

	// send initial state
	go h.bootstrapSubscriber(ch)

	closeFn := func() {
		h.evtMu.Lock()
		defer h.evtMu.Unlock()

		if _, exists := h.subs[ch]; exists {
			delete(h.subs, ch)
			close(ch)
		}
	}
	return ch, closeFn, nil
}

// Paths returns all note paths, with tmp layer taking precedence.
func (h *Hybrid) Paths() ([]Path, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	pPaths, err := h.pst.paths()
	if err != nil {
		return nil, fmt.Errorf("Paths: failed to get persistent paths: %w", err)
	}
	tPaths, err := h.tmp.paths()
	if err != nil {
		return nil, fmt.Errorf("Paths: failed to get tmp paths: %w", err)
	}
	uniq := make(map[Path]struct{})
	for _, p := range pPaths {
		uniq[p] = struct{}{}
	}
	for _, t := range tPaths {
		uniq[t] = struct{}{}
	}
	out := make([]Path, 0, len(uniq))
	for p := range uniq {
		out = append(out, p)
	}
	return out, nil
}

// ForwardLinks returns outgoing links, preferring temporary layer.
func (h *Hybrid) ForwardLinks(path Path) ([]Link, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.forwardLinks(path)
}

func (h *Hybrid) forwardLinks(path Path) ([]Link, error) {
	if n, ok := h.tmp.info(path); ok && !n.missing {
		return h.tmp.forwardLinks(path)
	}
	return h.pst.forwardLinks(path)
}

// BackLinks returns incoming links, merging layers with tmp overrides.
func (h *Hybrid) BackLinks(path Path) ([]Link, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.backLinks(path)
}

func (h *Hybrid) backLinks(path Path) ([]Link, error) {
	tmpLinks, err := h.tmp.backLinks(path)
	if err != nil {
		return nil, fmt.Errorf("backLinks tmp: %w", err)
	}
	pstLinks, err := h.pst.backLinks(path)
	if err != nil {
		return nil, fmt.Errorf("backLinks pst: %w", err)
	}
	var filtered []Link
	for _, l := range pstLinks {
		if n, ok := h.tmp.info(l.Src); !ok || n.missing {
			filtered = append(filtered, l)
		}
	}
	return append(tmpLinks, filtered...), nil
}

// Timestamp returns the timestamp of a persistent note.
func (h *Hybrid) Timestamp(path Path) (time.Time, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if n, ok := h.pst.info(path); ok {
		return n.Timestamp, nil
	}
	return time.Time{}, ErrNoteNotFound
}

// delete marks a note missing and cleans up.
func (h *Hybrid) delete(layer layer, path Path) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	// ensure exists
	if _, ok := layer.info(path); !ok {
		return fmt.Errorf("%w: %s", ErrNoteNotFound, path)
	}
	return h.withChangeDetection(path, layer, func() error {
		// mark missing & remove links
		return layer.upsert(note{Path: path, missing: true}, nil)
	})
}

// apply handles upsert with change detection.
func (h *Hybrid) apply(layer layer, n note, links []Link) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	// ensure index for note
	h.assignIndex(n.Path)
	// ensure targets exist
	for _, l := range links {
		if err := h.ensureTarget(n.Path, l, layer); err != nil {
			return err
		}
	}
	// perform upsert with change detection
	return h.withChangeDetection(n.Path, layer, func() error {
		return layer.upsert(n, links)
	})
}

// withChangeDetection captures link state, invokes op, then diffs and emits events.
func (h *Hybrid) withChangeDetection(path Path, layer layer, op func() error) error {
	// capture state
	prev, err := h.forwardLinks(path)
	if err != nil {
		return fmt.Errorf("changeDetection: prev forwardLinks: %w", err)
	}
	prevLayer, err := layer.forwardLinks(path)
	if err != nil {
		return fmt.Errorf("changeDetection: prevLayer forwardLinks: %w", err)
	}
	prevMissing := h.noteMissing(path)
	// execute operation
	if err := op(); err != nil {
		return err
	}
	// ensure index
	h.assignIndex(path)

	// record new missing-status & emit if changed
	newMissing := h.noteMissing(path)
	if prevMissing != newMissing {
		log.Printf("Missing status of note %s changed!", string(path))
		h.send(Event{
			Operation: "updateNote",
			Note:      NoteData{ID: int(h.idx[path]), Path: string(path), Missing: newMissing},
		})
	}

	// capture new state
	newLinks, err := h.forwardLinks(path)
	if err != nil {
		return fmt.Errorf("changeDetection: new forwardLinks: %w", err)
	}
	newLayer, err := layer.forwardLinks(path)
	if err != nil {
		return fmt.Errorf("changeDetection: newLayer forwardLinks: %w", err)
	}

	// detect special reindex for tmp layer
	special := layer == h.tmp && h.tryReindex(path, prev, newLinks)

	// emit link events
	h.emitDiff(path, prev, newLinks, !special)

	// cleanup orphans in this layer
	removed, _, _ := diff(path, prevLayer, newLayer)
	for _, tgt := range removed {
		if err := h.removeIndexIfOrphan(tgt, layer); err != nil {
			log.Printf("orphan cleanup error: %v", err)
		}
	}
	return nil
}

// assignIndex assigns a new ID and emits createNote if needed.
func (h *Hybrid) assignIndex(p Path) {
	if _, ok := h.idx[p]; !ok {
		h.counter++
		h.idx[p] = h.counter
		h.send(
			Event{
				Operation: "createNote",
				Note:      NoteData{ID: int(h.counter), Path: string(p), Missing: h.noteMissing(p)},
			},
		)
	}
}

// emitDiff emits deleteLink/createLink events based on diff slices.
func (h *Hybrid) emitDiff(path Path, old, new []Link, send bool) {
	rem, add, _ := diff(path, old, new)
	srcID := int(h.idx[path])
	for _, r := range rem {
		if send {
			h.send(
				Event{
					Operation: "deleteLink",
					Link:      LinkData{SourceID: srcID, TargetID: int(h.idx[r])},
				},
			)
		}
		// orphan removal handled elsewhere
	}
	for _, a := range add {
		if send {
			h.send(
				Event{
					Operation: "createLink",
					Link:      LinkData{SourceID: srcID, TargetID: int(h.idx[a])},
				},
			)
		}
	}
}

// tryReindex attempts the special moveIndex logic, returns true if applied.
func (h *Hybrid) tryReindex(path Path, old, new []Link) bool {
	rem, add, _ := diff(path, old, new)
	if len(rem) != 1 || len(add) != 1 {
		return false
	}
	r, a := rem[0], add[0]
	// only reindex missing->existing
	if info, ok := h.noteInfo(a); !ok || !info.missing {
		return false
	}
	// unique backlink
	if bl, err := h.backLinks(r); err != nil || len(bl) != 1 {
		return false
	}
	// perform move
	h.moveIndex(r, a)
	return true
}

// send broadcasts an event to all subscribers (non-blocking).
func (h *Hybrid) send(e Event) {
	h.evtMu.RLock()
	defer h.evtMu.RUnlock()
	for ch := range h.subs {
		ch <- e
	}
}

// bootstrapSubscriber streams current state to a new subscriber.
func (h *Hybrid) bootstrapSubscriber(chat chan Event) {
	h.mu.RLock()
	paths, err := h.Paths()
	if err != nil {
		log.Printf("bootstrap subscriber Paths error: %v", err)
		h.mu.RUnlock()
		return
	}
	h.mu.RUnlock()

	for _, p := range paths {
		n, ok := h.noteInfo(p)
		if !ok {
			continue
		}
		id := int(h.idx[p])
		chat <- Event{Operation: "createNote", Note: NoteData{ID: id, Path: string(p), Missing: n.missing}}
		links, err := h.forwardLinks(p)
		if err != nil {
			log.Printf("bootstrap subscriber forwardLinks error for %s: %v", p, err)
			continue
		}
		for _, l := range links {
			chat <- Event{Operation: "createLink", Link: LinkData{SourceID: id, TargetID: int(h.idx[l.Tgt])}}
		}
	}
}

// noteInfo retrieves a note from tmp or pst.
func (h *Hybrid) noteInfo(p Path) (note, bool) {
	if n, ok := h.tmp.info(p); ok {
		n.missing = h.noteMissing(p)
		return n, true
	}
	if n, ok := h.pst.info(p); ok {
		n.missing = h.noteMissing(p)
		return n, true
	}

	return note{}, false
}

// noteMissing checks if a note is marked missing.
func (h *Hybrid) noteMissing(p Path) bool {
	tmpNote, tmpOk := h.tmp.info(p)
	pstNote, pstOk := h.pst.info(p)

	if tmpOk && !tmpNote.missing {
		return false
	}
	if pstOk && !pstNote.missing {
		return false
	}

	return true
}

// ensureTarget ensures the link target exists in the given layer.
func (h *Hybrid) ensureTarget(src Path, l Link, layer layer) error {
	if l.Src != src {
		return fmt.Errorf("%w: mismatched source", ErrInvalidLink)
	}
	if l.Tgt == src {
		return nil
	}
	if _, ok := layer.info(l.Tgt); ok {
		return nil
	}
	if err := layer.upsert(note{Path: l.Tgt, missing: true}, nil); err != nil {
		return fmt.Errorf("creating missing note %s: %w", l.Tgt, err)
	}
	h.assignIndex(l.Tgt)
	return nil
}

// moveIndex transfers an ID from old path to new path and emits updateNote.
func (h *Hybrid) moveIndex(old, new Path) {
	id := h.idx[old]
	delete(h.idx, old)
	h.idx[new] = id
	h.send(
		Event{
			Operation: "updateNote",
			Note:      NoteData{ID: int(id), Path: string(new), Missing: true},
		},
	)
}

// removeIndexIfOrphan deletes missing notes without backlinks.
func (h *Hybrid) removeIndexIfOrphan(p Path, layer layer) error {
	n, ok := layer.info(p)
	if !ok || !n.missing {
		return nil
	}
	bl, err := layer.backLinks(p)
	if err != nil {
		return fmt.Errorf("orphan check backLinks: %w", err)
	}
	if len(bl) > 0 {
		return nil
	}
	if err := layer.delete(p); err != nil {
		return fmt.Errorf("orphan delete: %w", err)
	}
	// if gone from both layers
	if _, exists := h.noteInfo(p); !exists {
		if id, ok := h.idx[p]; ok {
			h.send(Event{Operation: "deleteNote", Note: NoteData{ID: int(id)}})
			delete(h.idx, p)
		}
	}
	return nil
}
