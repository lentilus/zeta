package cache

import "fmt"

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
