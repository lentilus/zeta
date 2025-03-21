package cache

import (
	"fmt"
	"sync"
)

type noteID uint // for identification in cytoscape only.

type PersistentCache struct {
	tmp     cacheLayer
	pst     cacheLayer
	idx     map[Path]noteID
	mu      sync.RWMutex
	counter noteID
}

func (ps *PersistentCache) index(path Path) {
	if _, ok := ps.idx[path]; ok {
		return
	}
	ps.counter += 1
	ps.idx[path] = ps.counter
}

func (ps *PersistentCache) moveIndex(start Path, dest Path) {
	// assert that the path has no backlinks.
	// point an index to a changed path

	// see upsert for details
}

// diff computes the difference in outgoing links from a path
func diff(path Path, a, b []Link) ([]Path, []Path, error) {
	fromA := make(map[Path]bool)
	onlyA := []Path{}

	fromB := make(map[Path]bool)
	onlyB := []Path{}

	for _, link := range a {
		// Check that the source of the links matches the path.
		if link.Src != path {
			return []Path{}, []Path{}, fmt.Errorf(
				"invalid link: src %s does not match note path %s",
				link.Src,
				path,
			)
		}
		fromA[link.Tgt] = true

	}

	for _, link := range b {
		// Check that the source of the links matches the path.
		if link.Src != path {
			return []Path{}, []Path{}, fmt.Errorf(
				"invalid link: src %s does not match note path %s",
				link.Src,
				path,
			)
		}
		fromB[link.Tgt] = true
		if !fromA[link.Tgt] {
			onlyB = append(onlyB, link.Tgt)
		}
	}

	for _, link := range a {
		if !fromB[link.Tgt] {
			onlyA = append(onlyA, link.Tgt)
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
		// remove no need for diff, since its entirely new
		// TODO: implementation
		return nil
	}

	currentLinks, err := ps.tmp.forwardLinks(note.Path)
	if err != nil {
		return err
	}

	willRemove, willAdd, err := diff(note.Path, currentLinks, links)
	// check if both are len1
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

	// Validate each link.
	for _, link := range links {
		// Check that the source of the link matches the note's path.
		if link.Src != note.Path {
			return fmt.Errorf(
				"invalid link %s: src %s does not match note path %s",
				link.Ref,
				link.Src,
				note.Path,
			)
		}

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
	ps.tmp.upsert(note, links)
	ps.index(note.Path)
	// TODO: push update for upserted note on channel
	// TODO: insert in idx

	return nil
}

func (ps *PersistentCache) BackLinks(path Path) ([]Link, error) {
	// TODO
}
