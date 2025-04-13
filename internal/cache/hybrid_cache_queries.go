package cache

import (
	"fmt"
	"log"
)

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
