package cache

import (
	"log"
	"testing"
)

func TestStuff(t *testing.T) {
	cache := NewPersistentCache()
	a := Note{Path: "a"}
	aLinks := []Link{
		{
			Src: "a",
			Row: 1,
			Col: 1,
			Ref: "[[b]]",
			Tgt: "b",
		},
	}

	err := cache.UpsertTmp(a, aLinks)
	if err != nil {
		panic(err)
	}
	log.Print(cache)
	log.Print(cache.idx)

	// b := Note{Path: "b"}
}
