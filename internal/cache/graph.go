package cache

import (
	"context"
	"sync"
)

// In-memory implementation of Cache interface.
// Uses maps for fast lookups.
type graph struct {
	mu          sync.RWMutex
	notes       map[Path]*Note
	forward     map[Path]map[Path]Link
	backlinks   map[Path]map[Path]Link
	subscribers map[int]chan Event
	nextSubID   int
}

// NewGraph creates a new in-memory Graph.
func NewGraph() Graph {
	return &graph{
		notes:       make(map[Path]*Note),
		forward:     make(map[Path]map[Path]Link),
		backlinks:   make(map[Path]map[Path]Link),
		subscribers: make(map[int]chan Event),
	}
}

// UpsertNote inserts or updates a note, diffing links and emitting events only on topology or metadata changes.
func (g *graph) UpsertNote(path Path, links []Link, metadata Metadata) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Ensure note exists (override placeholder)
	note, exists := g.notes[path]
	if !exists {
		note = &Note{Path: path, Placeholder: false, Metadata: metadata}
		g.notes[path] = note
		g.emit(
			Event{
				Type: CreateNote,
				Note: &NoteEvent{Path: path, Placeholder: false, Metadata: metadata},
			},
		)
	} else if note.Placeholder {
		note.Placeholder = false
		g.emit(Event{Type: UpdateNote, Note: &NoteEvent{Path: path, NewPath: path, Placeholder: false, Metadata: metadata}})
	} else {
		needsUpdate := false
		m := note.Metadata
		for k, v := range m {
			nv, _ := metadata[k]
			if nv != v {
				needsUpdate = true
				break
			}
		}
		m = metadata
		for k, v := range m {
			nv, _ := note.Metadata[k]
			if nv != v {
				needsUpdate = true
				break
			}
		}
		if needsUpdate {
			g.emit(Event{Type: UpdateNote, Note: &NoteEvent{Path: path, NewPath: path, Placeholder: false, Metadata: metadata}})
		}
	}

	// Build maps of old and new links
	oldLinks := g.forward[path]
	if oldLinks == nil {
		oldLinks = make(map[Path]Link)
	}
	newLinks := make(map[Path]Link, len(links))
	for _, l := range links {
		if l.Source != path {
			return ErrInvalidLink
		}
		newLinks[l.Target] = l
	}

	// Compute diffs: removals, additions, updates
	var toRemove []Path
	var toAdd, toUpdate []Link
	for tgt, newL := range newLinks {
		if _, found := oldLinks[tgt]; !found {
			toAdd = append(toAdd, newL)
		} else {
			toUpdate = append(toUpdate, newL)
		}
	}
	for tgt := range oldLinks {
		if _, found := newLinks[tgt]; !found {
			toRemove = append(toRemove, tgt)
		}
	}

	// Try placeholder-rename optimization
	if renameOptimized(path, toAdd, toRemove, g) {
		return nil
	}

	// Apply removals
	for _, tgt := range toRemove {
		deleteLink(path, tgt, g)
	}
	// Apply additions
	for _, l := range toAdd {
		createLink(path, l.Target, l, g)
	}
	// Apply range-only updates (no events)
	for _, l := range toUpdate {

		if g.forward[path] == nil {
			g.forward[path] = make(map[Path]Link)
		}
		g.forward[path][l.Target] = l
		if g.backlinks[l.Target] == nil {
			g.backlinks[l.Target] = make(map[Path]Link)
		}
		g.backlinks[l.Target][path] = l
	}

	return nil
}

// deleteLink removes a single link and emits events, cleaning up placeholders.
func deleteLink(src, tgt Path, g *graph) {
	delete(g.forward[src], tgt)
	if bl := g.backlinks[tgt]; bl != nil {
		delete(bl, src)
		if len(bl) == 0 {
			delete(g.backlinks, tgt)
		}
	}
	g.emit(Event{Type: DeleteLink, Link: &LinkEvent{Source: src, Target: tgt}})
	if ph, ok := g.notes[tgt]; ok && ph.Placeholder {
		if _, hasBack := g.backlinks[tgt]; !hasBack {
			delete(g.notes, tgt)
			g.emit(Event{Type: DeleteNote, Note: &NoteEvent{Path: tgt, Placeholder: true}})
		}
	}
}

// createLink adds a single link and emits events, creating placeholders as needed.
func createLink(src, tgt Path, l Link, g *graph) {
	if _, ok := g.notes[tgt]; !ok {
		g.notes[tgt] = &Note{Path: tgt, Placeholder: true}
		g.emit(Event{Type: CreateNote, Note: &NoteEvent{Path: tgt, Placeholder: true}})
	}
	if g.forward[src] == nil {
		g.forward[src] = make(map[Path]Link)
	}
	g.forward[src][tgt] = l
	if g.backlinks[tgt] == nil {
		g.backlinks[tgt] = make(map[Path]Link)
	}
	g.backlinks[tgt][src] = l
	g.emit(Event{Type: CreateLink, Link: &LinkEvent{Source: src, Target: tgt}})
}

// renameOptimized detects single-link placeholder renames and handles them.
func renameOptimized(path Path, add []Link, remove []Path, g *graph) bool {
	if len(add) == 1 && len(remove) == 1 {
		oT := remove[0]
		nT := add[0].Target
		ph, existed := g.notes[oT]
		_, newExists := g.notes[nT]
		bl := g.backlinks[oT]
		if existed && ph.Placeholder && !newExists && len(bl) == 1 {
			// remove the old forward/backlink
			delete(g.forward[path], oT)
			delete(g.backlinks[oT], path)

			// switch placeholder from oT to nT
			delete(g.notes, oT)
			g.notes[nT] = &Note{Path: nT, Placeholder: true}

			if g.forward[path] == nil {
				g.forward[path] = make(map[Path]Link)
			}
			g.forward[path][nT] = add[0]
			if g.backlinks[nT] == nil {
				g.backlinks[nT] = make(map[Path]Link)
			}
			g.backlinks[nT][path] = add[0]

			g.emit(
				Event{
					Type: UpdateNote,
					Note: &NoteEvent{Path: oT, NewPath: nT, Placeholder: true},
				},
			)
			return true
		}
	}
	return false
}

func (g *graph) DeleteNote(path Path) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	n, exists := g.notes[path]
	if !exists {
		return ErrNoteNotFound
	}
	if bl := g.backlinks[path]; bl != nil && len(bl) > 0 {
		if !n.Placeholder {
			n.Placeholder = true
			g.emit(
				Event{
					Type: UpdateNote,
					Note: &NoteEvent{Path: path, NewPath: path, Placeholder: true},
				},
			)
		}
		return nil
	}
	if fl := g.forward[path]; fl != nil {
		for tgt := range fl {
			delete(g.backlinks[tgt], path)
			g.emit(Event{Type: DeleteLink, Link: &LinkEvent{Source: path, Target: tgt}})
		}
		delete(g.forward, path)
	}
	delete(g.notes, path)
	g.emit(Event{Type: DeleteNote, Note: &NoteEvent{Path: path, Placeholder: n.Placeholder}})
	return nil
}

func (g *graph) GetPaths() []Path {
	g.mu.RLock()
	defer g.mu.RUnlock()
	paths := make([]Path, 0, len(g.notes))
	for p := range g.notes {
		paths = append(paths, p)
	}
	return paths
}

func (g *graph) GetForwardLinks(path Path) ([]Link, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	fl := g.forward[path]
	if fl == nil {
		return nil, nil
	}
	out := make([]Link, 0, len(fl))
	for _, l := range fl {
		out = append(out, l)
	}
	return out, nil
}

func (g *graph) GetBackLinks(path Path) ([]Link, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	bl := g.backlinks[path]
	if bl == nil {
		return nil, nil
	}
	out := make([]Link, 0, len(bl))
	for _, l := range bl {
		out = append(out, l)
	}
	return out, nil
}

func (g *graph) IsPlaceholder(path Path) (bool, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	note, ok := g.notes[path]
	if !ok {
		return false, ErrNoteNotFound
	}
	return note.Placeholder, nil
}

func (g *graph) Subscribe(ctx context.Context) (<-chan Event, error) {
	g.mu.Lock()
	ch := make(chan Event, 16)
	sid := g.nextSubID
	g.nextSubID++
	g.subscribers[sid] = ch

	notes := make([]*NoteEvent, 0, len(g.notes))
	for _, note := range g.notes {
		notes = append(
			notes,
			&NoteEvent{Path: note.Path, Placeholder: note.Placeholder, Metadata: note.Metadata},
		)
	}
	links := make([]*LinkEvent, 0)
	for src, targets := range g.forward {
		for tgt := range targets {
			links = append(links, &LinkEvent{Source: src, Target: tgt})
		}
	}
	g.mu.Unlock()

	go func() {
		for _, ne := range notes {
			select {
			case ch <- Event{Type: CreateNote, Note: ne}:
			case <-ctx.Done():
				return
			}
		}
		for _, le := range links {
			select {
			case ch <- Event{Type: CreateLink, Link: le}:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		<-ctx.Done()
		g.mu.Lock()
		delete(g.subscribers, sid)
		close(ch)
		g.mu.Unlock()
	}()

	return ch, nil
}

// emit sends events to all subscribers (non-blocking).
func (g *graph) emit(event Event) {
	for _, ch := range g.subscribers {
		ch <- event
	}
}
