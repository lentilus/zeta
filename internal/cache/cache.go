package cache

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

type Cache interface {
	SaveNote(path Path, forwardLinks []Link, metaData Metadata, saveTime time.Time) error
	EditNote(path Path, forwardLinks []Link, metaData Metadata) error
	DiscardNote(path Path) error
	DeleteNote(path Path) error
	GetPaths() []Path
	GetSaveTime(path Path) time.Time
	NoteExists(path Path) bool
	GetForwardLinks(path Path) ([]Link, error)
	GetBackLinks(path Path) ([]Link, error)
	GetMetaData(path Path) (Metadata, error)
	Subscribe(ctx context.Context) (<-chan Event, error)
	Dump() []byte
}

type cache struct {
	mu              sync.RWMutex
	graph           Graph              `json:"-"`
	SavedNotes      map[Path][]Link    `json:"saved_notes"`
	SaveTimes       map[Path]time.Time `json:"save_times"`
	SavedMetaData   map[Path]Metadata  `json:"metadata"`
	CurrentMetaData map[Path]Metadata  `json:"-"`
}

func NewCache() Cache {
	return &cache{
		graph:           NewGraph(),
		SavedNotes:      make(map[Path][]Link),
		SaveTimes:       make(map[Path]time.Time),
		SavedMetaData:   make(map[Path]Metadata),
		CurrentMetaData: make(map[Path]Metadata),
	}
}

// RestoreCache takes a JSON dump (produced by Dump) and rebuilds both the maps and the graph by replaying SaveNote.
func RestoreCache(dump []byte) (Cache, error) {
	c := cache{}
	if err := json.Unmarshal(dump, &c); err != nil {
		return nil, err
	}

	// ensure maps are non-nil
	if c.SavedNotes == nil {
		c.SavedNotes = make(map[Path][]Link)
	}
	if c.SaveTimes == nil {
		c.SaveTimes = make(map[Path]time.Time)
	}
	if c.SavedMetaData == nil {
		c.SavedMetaData = make(map[Path]Metadata)
	}
	// initialize current metadata from saved metadata
	c.CurrentMetaData = make(map[Path]Metadata, len(c.SavedMetaData))
	for path, m := range c.SavedMetaData {
		mCopy := make(Metadata, len(m))
		for k, v := range m {
			mCopy[k] = v
		}
		c.CurrentMetaData[path] = mCopy
	}

	c.graph = NewGraph()
	for path, links := range c.SavedNotes {
		_, ok := c.SaveTimes[path]
		if !ok {
			return nil, errors.New("missing save-time for " + string(path))
		}
		if err := c.graph.UpsertNote(path, links, c.SavedMetaData[path]); err != nil {
			return nil, err
		}
	}

	return &c, nil
}

// SaveNote commits a note's links, save time, and metadata.
func (c *cache) SaveNote(
	path Path,
	forwardLinks []Link,
	metaData Metadata,
	saveTime time.Time,
) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.graph.UpsertNote(path, forwardLinks, metaData); err != nil {
		return err
	}
	// commit links and time
	c.SavedNotes[path] = forwardLinks
	c.SaveTimes[path] = saveTime
	// commit metadata
	mCopy := make(map[string]string, len(metaData))
	for k, v := range metaData {
		mCopy[k] = v
	}
	c.SavedMetaData[path] = mCopy
	c.CurrentMetaData[path] = mCopy
	return nil
}

// EditNote updates a note's links and staging metadata without changing the save time or saved state.
func (c *cache) EditNote(path Path, forwardLinks []Link, metaData map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.graph.UpsertNote(path, forwardLinks, metaData); err != nil {
		return err
	}
	// update saved-links view
	c.SavedNotes[path] = forwardLinks
	// update staging metadata
	mCopy := make(map[string]string, len(metaData))
	for k, v := range metaData {
		mCopy[k] = v
	}
	c.CurrentMetaData[path] = mCopy
	return nil
}

func (c *cache) DeleteNote(path Path) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	err := c.graph.DeleteNote(path)
	if err != nil {
		return err
	}
	delete(c.SavedNotes, path)
	delete(c.SaveTimes, path)
	delete(c.SavedMetaData, path)
	delete(c.CurrentMetaData, path)
	return nil
}

// DiscardNote reverts links and metadata to the last saved state.
func (c *cache) DiscardNote(path Path) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var savedM Metadata
	if links, ok := c.SavedNotes[path]; ok {
		// restore metadata staging
		if savedM, ok2 := c.SavedMetaData[path]; ok2 {
			mCopy := make(map[string]string, len(savedM))
			for k, v := range savedM {
				mCopy[k] = v
			}
			c.CurrentMetaData[path] = mCopy
		} else {
			delete(c.CurrentMetaData, path)
		}

		// restore forward links
		if err := c.graph.UpsertNote(path, links, savedM); err != nil {
			return err
		}
		return nil
	}
	// no saved version: delete entirely
	err := c.graph.DeleteNote(path)
	if err != nil {
		return err
	}
	delete(c.CurrentMetaData, path)
	return nil
}

func (c *cache) GetPaths() []Path {
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
	return c.graph.GetForwardLinks(path)
}

func (c *cache) GetBackLinks(path Path) ([]Link, error) {
	return c.graph.GetBackLinks(path)
}

// GetMetaData returns the staging metadata map for a given note, or an error if not found.
func (c *cache) GetMetaData(path Path) (Metadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m, ok := c.CurrentMetaData[path]
	if !ok {
		return nil, errors.New("no metadata for " + string(path))
	}
	return m, nil
}

func (c *cache) Subscribe(ctx context.Context) (<-chan Event, error) {
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
