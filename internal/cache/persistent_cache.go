package cache

import (
	"fmt"
	"sync"
)

type noteID uint // for identification in cytoscape only.

type PersistentCache struct {
	tmp        cacheLayer
	pst        cacheLayer
	idx        map[Path]noteID
	idxCounter noteID
	mu         sync.RWMutex
}

// NewPersistentCache initializes and returns a new instance of PersistentCache.
func NewPersistentCache() *PersistentCache {
	return &PersistentCache{
		tmp:        newMapCacheLayer(),
		pst:        newMapCacheLayer(),
		idx:        make(map[Path]noteID),
		idxCounter: 0,
		mu:         sync.RWMutex{},
	}
}

func (ps *PersistentCache) index(path Path) {
	if _, ok := ps.idx[path]; ok {
		return
	}
	ps.idxCounter += 1
	ps.idx[path] = ps.idxCounter
}

func (ps *PersistentCache) moveIndex(start Path, dest Path) {
	// assert that the path has no backlinks.
	// point an index to a changed path

	// see upsert for details
}

// getTargets builds a map from target paths for the given links,
// ensuring that all links have the same source path.
func getTargets(path Path, links []Link) (map[Path]struct{}, error) {
	m := make(map[Path]struct{}, len(links))
	for _, link := range links {
		if link.Src != path {
			return nil, fmt.Errorf(
				"invalid link: src %s does not match note path %s",
				link.Src,
				path,
			)
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

// UpsertTmp updates or inserts a note into the temporary cache layer.
func (ps *PersistentCache) UpsertTmp(note Note, links []Link) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// set missing to false
	note.missing = false
	if _, ok := ps.pst.info(note.Path); !ok {
		// no need for diff, since its entirely new
		fmt.Printf("\"%s\" is new.\n", note.Path)

		for _, link := range links {
			// If the target is not the note itself, check that it exists.
			// otherwise insert as missing note
			if link.Tgt != note.Path {
				if _, ok := ps.pst.info(note.Path); !ok {
					ps.tmp.upsert(Note{Path: link.Tgt, missing: true}, []Link{})
					ps.index(link.Tgt)
					// TODO: push update for new notes on channel
				}
			}
		}

		err := ps.tmp.upsert(note, links)
		if err != nil {
			return err
		}
		ps.index(note.Path)

		return nil
	}

	//
	currentLinks, err := ps.tmp.forwardLinks(note.Path)
	if err != nil {
		return err
	}

	willRemove, willAdd, err := diff(note.Path, currentLinks, links)

	// 1. No topological changes
	if len(willRemove) == 0 && len(willAdd) == 0 {
		err := ps.tmp.upsert(note, links)
		if err != nil {
			return err
		}
		ps.index(note.Path)
		return nil
	}

	// 2. Minor change (moved Link)
	/*
		if len(willRemove) == 1 && len(willAdd) == 1 {
			// check that the one to remove does not exist (i.e is not in the idx)
			// and that the one to add does not exist (neither in tmp nor in pst)
			// and that the one to remove does not have backlinks (use ps.BackLinks)
			// -> perform "move"

			// this indicates that the link is currently being typed
			// NOTE: We will use this for a smoother looking graph update in cytoscape.
			// We want to display the path of each node in realtime, and it looks
			// nice, if we can update the path of a node as it is typed.
			// This is the reason, why nodes in cytoscape get identified via an id
			// instead of using the path as the uid directly.
		}
	*/

	// 3. Substantial change
	for _, link := range links {
		// If the target is not the note itself, check that it exists.
		// otherwise insert as missing note
		if link.Tgt != note.Path {
			if _, ok := ps.pst.info(note.Path); !ok {
				ps.tmp.upsert(Note{Path: link.Tgt, missing: true}, []Link{})
				ps.index(link.Tgt)
				// TODO: push update for new notes on channel
			}
		}
	}
	err = ps.tmp.upsert(note, links)
	if err != nil {
		return err
	}
	ps.index(note.Path)
	// TODO: push update for upserted note on channel
	// TODO: insert in idx

	return nil
}

/*
func (ps *PersistentCache) BackLinks(path Path) ([]Link, error) {
	// TODO
}
*/
