package cache

import (
	"log"
	"testing"
    "fmt"
)

func TestStuff(t *testing.T) {
    pc := NewHybridCache()

	// Step 1: Insert a note "A" linking to "B" and "C"
	noteA := Note{Path: "A"}
	linksA := []Link{
		{Src: "A", Tgt: "B"},
		{Src: "A", Tgt: "C"},
	}

	if err := pc.Upsert(noteA, linksA); err != nil {
		log.Fatalf("Error upserting note A: %v", err)
	}
	fmt.Println("Added note A with links to B and C.")
    fmt.Println(pc.sprint())

	// Step 2: Insert note "B" with a link to "C"
	noteB := Note{Path: "B"}
	linksB := []Link{
		{Src: "B", Tgt: "C"},
	}
	if err := pc.UpsertTmp(noteB, linksB); err != nil {
		log.Fatalf("Error upserting note B: %v", err)
	}
	fmt.Println("[Tmp] Added note B with a link to C.")
    fmt.Println(pc.sprint())

	// Step 3: Retrieve and print all stored paths
	paths, err := pc.Paths()
	if err != nil {
		log.Fatalf("Error retrieving paths: %v", err)
	}
	fmt.Println("All notes in cache:", paths)
    fmt.Println(pc.sprint())

	// Step 4: Retrieve and print forward links from A
	links, err := pc.ForwardLinks("A")
	if err != nil {
		log.Fatalf("Error retrieving forward links for A: %v", err)
	}
	fmt.Println("Links from A:", links)
    fmt.Println(pc.sprint())

	// Step 5: Update note A to remove link to C and add link to D
	updatedLinksA := []Link{
		{Src: "A", Tgt: "D"},
	}
	if err := pc.Upsert(noteA, updatedLinksA); err != nil {
		log.Fatalf("Error updating note A: %v", err)
	}
	fmt.Println("Updated note A: now links to D instead of C.")
    fmt.Println(pc.sprint())

	// Step 6: Retrieve and print updated forward links from A
	links, err = pc.ForwardLinks("A")
	if err != nil {
		log.Fatalf("Error retrieving updated forward links for A: %v", err)
	}
	fmt.Println("Updated links from A:", links)
    fmt.Println(pc.sprint())

	// Step 7: Delete note B
	if err := pc.DeleteTmp(noteB.Path); err != nil {
		log.Fatalf("Error deleting note B: %v", err)
	}
	fmt.Println("Deleted note B.")
    fmt.Println(pc.sprint())

	// Step 8: Retrieve and print final paths
	paths, err = pc.Paths()
	if err != nil {
		log.Fatalf("Error retrieving final paths: %v", err)
	}
	fmt.Println("Final notes in cache:", paths)
    fmt.Println(pc.sprint())

	// Step 9: Create C with link to D
	updatedLinksC := []Link{
		{Src: "C", Tgt: "D"},
	}
	noteC := Note{Path: "C"}
	if err := pc.Upsert(noteC, updatedLinksC); err != nil {
		log.Fatalf("Error inserting note : C %v", err)
	}
	fmt.Println("Create C:  links to D")
    fmt.Println(pc.sprint())

	// Step 10: Delete note A
    if err := pc.Delete("A"); err != nil {
		log.Fatalf("Error deleting note A: %v", err)
	}
	fmt.Println("Deleted note A.")
    fmt.Println(pc.sprint())
}
