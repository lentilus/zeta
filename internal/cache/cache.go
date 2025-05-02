package cache

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

type Cache interface {
	SaveNote(path Path, forwardLinks []Link, saveTime time.Time) error
	EditNote(path Path, forwardLinks []Link) error
	DiscardNote(path Path) error
	GetPaths() []Path
	GetSaveTime(path Path) time.Time
	NoteExists(path Path) bool
	GetForwardLinks(path Path) ([]Link, error)
	GetBackLinks(path Path) ([]Link, error)
	Subscribe(ctx context.Context) (<-chan Event, error)
	Dump() []byte
}

type cache struct {
	mu         sync.RWMutex
	graph      Graph              `json:"-"`
	SavedNotes map[Path][]Link    `json:"saved_notes"`
	SaveTimes  map[Path]time.Time `json:"save_times"`
}

func NewCache() Cache {
	return &cache{
		graph:      NewGraph(),
		SavedNotes: make(map[Path][]Link),
		SaveTimes:  make(map[Path]time.Time),
	}
}

// RestoreCache takes a JSON dump (produced by Dump) and rebuilds both the maps and the graph by replaying SaveNote.
func RestoreCache(dump []byte) (Cache, error) {
	c := cache{}
	if err := json.Unmarshal(dump, &c); err != nil {
		return nil, err
	}

	c.graph = NewGraph()

	for path, links := range c.SavedNotes {
		_, ok := c.SaveTimes[path]
		if !ok {
			return nil, errors.New("missing save-time for " + string(path))
		}
		if err := c.graph.UpsertNote(path, links); err != nil {
			return nil, err
		}
		// SaveTimes already set by unmarshal
	}

	return &c, nil
}

func (c *cache) SaveNote(path Path, forwardLinks []Link, saveTime time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.graph.UpsertNote(path, forwardLinks); err != nil {
		return err
	}
	c.SavedNotes[path] = forwardLinks
	c.SaveTimes[path] = saveTime
	return nil
}

func (c *cache) EditNote(path Path, forwardLinks []Link) error {
	// no lock needed; graph handles its own concurrency
	return c.graph.UpsertNote(path, forwardLinks)
}

func (c *cache) DiscardNote(path Path) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if links, ok := c.SavedNotes[path]; ok {
		return c.graph.UpsertNote(path, links)
	}
	return c.graph.DeleteNote(path)
}

func (c *cache) GetPaths() []Path {
	// no lock needed; graph handles its own concurrency
	return c.graph.GetPaths()
}

func (c *cache) NoteExists(path Path) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	placeholder, err := c.graph.IsPlaceholder(path)
	if err != nil {
		return false
	}
	return !placeholder
}

func (c *cache) GetSaveTime(path Path) time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	t, ok := c.SaveTimes[path]
	if !ok {
		return time.Time{}
	}
	return t
}

func (c *cache) GetForwardLinks(path Path) ([]Link, error) {
	// no lock needed; graph handles its own concurrency
	return c.graph.GetForwardLinks(path)
}

func (c *cache) GetBackLinks(path Path) ([]Link, error) {
	// no lock needed; graph handles its own concurrency
	return c.graph.GetBackLinks(path)
}

func (c *cache) Subscribe(ctx context.Context) (<-chan Event, error) {
	// no lock needed; graph handles its own concurrency
	return c.graph.Subscribe(ctx)
}

func (c *cache) Dump() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()

	dump, err := json.Marshal(c)
	if err != nil {
		return nil
	}
	return dump
}
