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

// NewCache creates a new in-memory Cache.
func NewGraph() Graph {
	return &graph{
		notes:       make(map[Path]*Note),
		forward:     make(map[Path]map[Path]Link),
		backlinks:   make(map[Path]map[Path]Link),
		subscribers: make(map[int]chan Event),
	}
}

func (g *graph) UpsertNote(path Path, links []Link) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Ensure note exists (override placeholder)
	note, exists := g.notes[path]
	if !exists {
		note = &Note{Path: path, Placeholder: false}
		g.notes[path] = note
		g.emit(Event{Type: CreateNote, Note: &NoteEvent{Path: path, Placeholder: false}})
	} else if note.Placeholder {
		note.Placeholder = false
		g.emit(Event{Type: UpdateNote, Note: &NoteEvent{Path: path, Placeholder: false}})
	}

	// Build sets of existing and new targets
	existing := g.forward[path]
	newSet := make(map[Path]Link, len(links))
	for _, l := range links {
		if l.Source != path {
			return ErrInvalidLink
		}
		newSet[l.Target] = l
	}

	// Try to detect placeholder rename optimization
	if func() bool {
		if len(existing) != 1 || len(newSet) != 1 {
			return false
		}
		var oldT, newT Path
		for t := range existing {
			oldT = t
		}
		for t := range newSet {
			newT = t
		}
		ph, oldExists := g.notes[oldT]
		_, newExists := g.notes[newT]
		bl := g.backlinks[oldT]
		if !oldExists || !ph.Placeholder || newExists || (bl != nil && len(bl) > 0) {
			return false
		}
		// rename placeholder without link events
		delete(g.notes, oldT)
		delete(g.backlinks, oldT)
		g.notes[newT] = &Note{Path: newT, Placeholder: true}
		g.forward[path] = map[Path]Link{newT: newSet[newT]}
		g.backlinks[newT] = map[Path]Link{path: newSet[newT]}
		g.emit(Event{Type: UpdateNote, Note: &NoteEvent{Path: newT, Placeholder: true}})
		return true
	}() {
		return nil
	}

	// Remove all old links
	for tgt := range existing {
		delete(g.forward[path], tgt)
		if bl := g.backlinks[tgt]; bl != nil {
			delete(bl, path)
			if len(bl) == 0 {
				delete(g.backlinks, tgt)
			}
		}
		g.emit(Event{Type: DeleteLink, Link: &LinkEvent{Source: path, Target: tgt}})
		// drop orphan placeholder
		if ph, ok := g.notes[tgt]; ok && ph.Placeholder {
			if _, hasBack := g.backlinks[tgt]; !hasBack {
				delete(g.notes, tgt)
				g.emit(Event{Type: DeleteNote, Note: &NoteEvent{Path: tgt, Placeholder: true}})
			}
		}
	}

	// Add all new links
	g.forward[path] = make(map[Path]Link, len(newSet))
	for tgt, l := range newSet {
		// placeholder if missing
		if _, ok := g.notes[tgt]; !ok {
			g.notes[tgt] = &Note{Path: tgt, Placeholder: true}
			g.emit(Event{Type: CreateNote, Note: &NoteEvent{Path: tgt, Placeholder: true}})
		}
		g.forward[path][tgt] = l
		if g.backlinks[tgt] == nil {
			g.backlinks[tgt] = make(map[Path]Link)
		}
		g.backlinks[tgt][path] = l
		g.emit(Event{Type: CreateLink, Link: &LinkEvent{Source: path, Target: tgt}})
	}
	return nil
}

func (g *graph) DeleteNote(path Path) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	// check existence
	n, exists := g.notes[path]
	if !exists {
		return ErrNoteNotFound
	}
	// if has backlinks, mark placeholder (missing)
	if bl := g.backlinks[path]; bl != nil && len(bl) > 0 {
		if !n.Placeholder {
			n.Placeholder = true
			g.emit(Event{Type: UpdateNote, Note: &NoteEvent{Path: path, Placeholder: true}})
		}
		return nil
	}
	// safe to delete
	// remove forward links
	if fl := g.forward[path]; fl != nil {
		for tgt := range fl {
			delete(g.backlinks[tgt], path)
			g.emit(Event{Type: DeleteLink, Link: &LinkEvent{Source: path, Target: tgt}})
		}
		delete(g.forward, path)
	}
	// remove note
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
	g.mu.Unlock()

	// remove on context done
	go func() {
		<-ctx.Done()
		g.mu.Lock()
		delete(g.subscribers, sid)
		close(ch)
		g.mu.Unlock()
	}()
	return ch, nil
}

// emit sends event to all subscribers non-blocking
func (g *graph) emit(event Event) {
	for _, ch := range g.subscribers {
		select {
		case ch <- event:
		default:
			// drop if not ready
		}
	}
}
