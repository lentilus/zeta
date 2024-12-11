package store

import (
	"fmt"
	"sync"
	"time"
)

// DummyStore implements the Store interface with in-memory storage
type DummyStore struct {
	// links stores the relationships between zettels
	// map[source][]target for forward links
	links map[string][]string
	// reverseLinks stores backward references
	// map[target][]source for backward links
	reverseLinks map[string][]string
	// lastUpdate tracks when each zettel was last updated
	lastUpdate map[string]time.Time
	mu         sync.RWMutex
}

// NewDummyStore creates a new DummyStore with some sample data
func NewDummyStore() *DummyStore {
	store := &DummyStore{
		links:        make(map[string][]string),
		reverseLinks: make(map[string][]string),
		lastUpdate:   make(map[string]time.Time),
	}

	// Add some sample data
	samplePaths := []string{
		"zettel1.typ",
		"zettel2.typ",
		"zettel3.typ",
		"folder/zettel4.typ",
		"folder/zettel5.typ",
	}

	// Create some sample relationships
	store.links[samplePaths[0]] = []string{samplePaths[1], samplePaths[2]}
	store.links[samplePaths[1]] = []string{samplePaths[2], samplePaths[3]}
	store.links[samplePaths[2]] = []string{samplePaths[4]}

	// Build reverse links
	for source, targets := range store.links {
		for _, target := range targets {
			store.reverseLinks[target] = append(store.reverseLinks[target], source)
		}
	}

	// Set initial update times
	now := time.Now()
	for _, path := range samplePaths {
		store.lastUpdate[path] = now
	}

	return store
}

// UpdateOne simulates updating a single zettel
func (s *DummyStore) UpdateOne(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Simulate some processing time
	time.Sleep(100 * time.Millisecond)

	if _, exists := s.lastUpdate[path]; !exists {
		return fmt.Errorf("zettel not found: %s", path)
	}

	// Update the last update time
	s.lastUpdate[path] = time.Now()
	return nil
}

// UpdateAll simulates updating all zettels
func (s *DummyStore) UpdateAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Simulate some processing time
	time.Sleep(500 * time.Millisecond)

	now := time.Now()
	for path := range s.lastUpdate {
		s.lastUpdate[path] = now
	}
	return nil
}

// Recompute simulates recomputing all relationships
func (s *DummyStore) Recompute() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Simulate intensive processing
	time.Sleep(1 * time.Second)

	// Clear and rebuild reverse links
	s.reverseLinks = make(map[string][]string)
	for source, targets := range s.links {
		for _, target := range targets {
			s.reverseLinks[target] = append(s.reverseLinks[target], source)
		}
	}
	return nil
}

// GetAll returns all zettel paths
func (s *DummyStore) GetAll() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	paths := make([]string, 0, len(s.lastUpdate))
	for path := range s.lastUpdate {
		paths = append(paths, path)
	}
	return paths, nil
}

// GetParents returns all zettels that link to the given path
func (s *DummyStore) GetParents(path string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.lastUpdate[path]; !exists {
		return nil, fmt.Errorf("zettel not found: %s", path)
	}

	parents := make([]string, len(s.reverseLinks[path]))
	copy(parents, s.reverseLinks[path])
	return parents, nil
}

// GetChildren returns all zettels that the given path links to
func (s *DummyStore) GetChildren(path string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, exists := s.lastUpdate[path]; !exists {
		return nil, fmt.Errorf("zettel not found: %s", path)
	}

	children := make([]string, len(s.links[path]))
	copy(children, s.links[path])
	return children, nil
}
