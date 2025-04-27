package cache_test

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"
	"zeta/internal/cache"
)

// op represents a single cache operation.
type op struct {
	typ   int // 0=Upsert,1=UpsertTmp,2=Delete,3=DeleteTmp
	path  cache.Path
	links []cache.Link
	ts    time.Time
}

// parseOps builds a sequence of ops from fuzz input bytes.
func parseOps(data []byte) []op {
	const maxPaths = 8
	n := len(data) / 3
	ops := make([]op, 0, n)
	for i := 0; i < n; i++ {
		b0 := data[i*3]
		b1 := data[i*3+1]
		b2 := data[i*3+2]
		typ := int(b0 % 4)
		p := cache.Path(fmt.Sprintf("path%d", int(b1)%maxPaths))
		var links []cache.Link
		if typ == 0 || typ == 1 {
			// one link target
			target := cache.Path(fmt.Sprintf("path%d", int(b2)%maxPaths))
			links = []cache.Link{{Src: p, Tgt: target}}
		}
		// assign a pseudo timestamp for persistent upserts
		ts := time.Unix(int64(b0), 0)
		ops = append(ops, op{typ: typ, path: p, links: links, ts: ts})
	}
	return ops
}

// apply runs ops against a fresh Hybrid and returns any error.
func apply(h *cache.Hybrid, ops []op) error {
	for _, o := range ops {
		switch o.typ {
		case 0:
			if err := h.Upsert(o.path, o.links, o.ts); err != nil {
				return err
			}
		case 1:
			if err := h.UpsertTmp(o.path, o.links); err != nil {
				return err
			}
		case 2:
			if err := h.Delete(o.path); err != nil {
				return err
			}
		case 3:
			if err := h.DeleteTmp(o.path); err != nil {
				return err
			}
		}
	}
	return nil
}

// compareCaches uses only exported methods to assert equivalence.
func compareCaches(a, b *cache.Hybrid) bool {
	pa, _ := a.Paths()
	pb, _ := b.Paths()
	sa, sb := toStrings(pa), toStrings(pb)
	sort.Strings(sa)
	sort.Strings(sb)
	if !equal(sa, sb) {
		return false
	}
	for _, p := range sa {
		path := cache.Path(p)
		fa, _ := a.ForwardLinks(path)
		fb, _ := b.ForwardLinks(path)
		if !compareTargets(fa, fb) {
			return false
		}
		ba, _ := a.BackLinks(path)
		bb, _ := b.BackLinks(path)
		if !compareSources(ba, bb) {
			return false
		}
	}
	return true
}

func toStrings(ps []cache.Path) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = string(p)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func compareTargets(a, b []cache.Link) bool {
	aS, bS := []string{}, []string{}
	for _, l := range a {
		aS = append(aS, string(l.Tgt))
	}
	for _, l := range b {
		bS = append(bS, string(l.Tgt))
	}
	sort.Strings(aS)
	sort.Strings(bS)
	return equal(aS, bS)
}

func compareSources(a, b []cache.Link) bool {
	aS, bS := []string{}, []string{}
	for _, l := range a {
		aS = append(aS, string(l.Src))
	}
	for _, l := range b {
		bS = append(bS, string(l.Src))
	}
	sort.Strings(aS)
	sort.Strings(bS)
	return equal(aS, bS)
}

// FuzzOperationOrder leverages Go's fuzzing engine to find non-commutative sequences,
// but only for non-conflicting operations on distinct paths.
func FuzzOperationOrder(f *testing.F) {
	f.Add([]byte{0, 1, 2, 1, 2, 3, 2, 3, 4})
	f.Fuzz(func(t *testing.T, data []byte) {
		ops := parseOps(data)
		// require at least two operations
		if len(ops) < 2 {
			t.Skip()
		}
		// restrict to distinct paths—no path may appear twice
		seen := make(map[cache.Path]struct{})
		for _, o := range ops {
			if _, ok := seen[o.path]; ok {
				t.Skip()
			}
			seen[o.path] = struct{}{}
		}

		// apply original sequence
		h1 := cache.NewHybrid()
		if err := apply(h1, ops); err != nil {
			t.Skip()
		}

		// shuffle and reapply
		op2 := make([]op, len(ops))
		copy(op2, ops)
		rand.Shuffle(len(op2), func(i, j int) { op2[i], op2[j] = op2[j], op2[i] })

		h2 := cache.NewHybrid()
		if err := apply(h2, op2); err != nil {
			t.Skip()
		}

		// compare final state; should be equal for non-conflicting ops
		if !compareCaches(h1, h2) {
			t.Fatalf("non-commutative for distinct-path ops: %v vs %v", ops, op2)
		}
	})
}
