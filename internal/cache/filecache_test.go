package cache_test

import (
	"path/filepath"
	"testing"
	"time"

	"zeta/internal/cache" // Adjust import path as needed
)

// newTestCache creates a new cache instance with a temporary database file.
func newTestCache(t *testing.T) cache.Cache {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	c, err := cache.NewFilecache(dbPath)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	return c
}

func TestNewFilecache(t *testing.T) {
	c := newTestCache(t)
	if c == nil {
		t.Fatal("expected cache not to be nil")
	}
}

func TestUpsertNote(t *testing.T) {
	c := newTestCache(t)
	note := cache.Note("note1")
	link := cache.Link{
		Reference: "link1",
		Source:    "note1",
		Target:    "note2",
		Row:       1,
		Col:       1,
	}

	if err := c.UpsertNote(note, []cache.Link{link}); err != nil {
		t.Fatalf("UpsertNote failed: %v", err)
	}

	// Verify that the note appears in GetAllNotes.
	notes, err := c.GetAllNotes()
	if err != nil {
		t.Fatalf("GetAllNotes failed: %v", err)
	}
	found := false
	for _, n := range notes {
		if n == note {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("UpsertNote did not add note to GetAllNotes")
	}

	// Verify forward links.
	links, err := c.GetForwardLinks(note)
	if err != nil {
		t.Fatalf("GetForwardLinks failed: %v", err)
	}
	if len(links) != 1 {
		t.Errorf("expected 1 forward link, got %d", len(links))
	} else {
		if links[0] != link {
			t.Errorf("forward link does not match expected values")
		}
	}
}

func TestDeleteNote(t *testing.T) {
	c := newTestCache(t)
	note := cache.Note("note_to_delete")
	if err := c.UpsertNote(note, nil); err != nil {
		t.Fatalf("UpsertNote failed: %v", err)
	}
	if err := c.DeleteNote(note); err != nil {
		t.Fatalf("DeleteNote failed: %v", err)
	}
	notes, err := c.GetAllNotes()
	if err != nil {
		t.Fatalf("GetAllNotes failed: %v", err)
	}
	for _, n := range notes {
		if n == note {
			t.Errorf("DeleteNote did not remove note")
		}
	}
}

func TestGetLastModified(t *testing.T) {
	c := newTestCache(t)
	note := cache.Note("note_last_modified")
	before := time.Now()
	if err := c.UpsertNote(note, nil); err != nil {
		t.Fatalf("UpsertNote failed: %v", err)
	}
	lastModified, err := c.GetLastModified(note)
	if err != nil {
		t.Fatalf("GetLastModified failed: %v", err)
	}
	if lastModified.Before(before) {
		t.Errorf("lastModified time is before insertion time")
	}
	// Update the note and check that lastModified changes.
	time.Sleep(1 * time.Second)
	if err := c.UpsertNote(note, nil); err != nil {
		t.Fatalf("UpsertNote failed on update: %v", err)
	}
	updated, err := c.GetLastModified(note)
	if err != nil {
		t.Fatalf("GetLastModified failed after update: %v", err)
	}
	if !updated.After(lastModified) {
		t.Errorf("lastModified was not updated on UpsertNote")
	}
}

func TestGetAllNotes(t *testing.T) {
	c := newTestCache(t)
	note1 := cache.Note("note1")
	note2 := cache.Note("note2")
	if err := c.UpsertNote(note1, nil); err != nil {
		t.Fatalf("UpsertNote failed for note1: %v", err)
	}
	if err := c.UpsertNote(note2, nil); err != nil {
		t.Fatalf("UpsertNote failed for note2: %v", err)
	}
	all, err := c.GetAllNotes()
	if err != nil {
		t.Fatalf("GetAllNotes failed: %v", err)
	}
	if len(all) < 2 {
		t.Errorf("expected at least 2 notes, got %d", len(all))
	}
}

func TestGetForwardLinks(t *testing.T) {
	c := newTestCache(t)
	note := cache.Note("forward_source")
	link := cache.Link{
		Reference: "fwd_ref",
		Source:    "forward_source",
		Target:    "forward_target",
		Row:       10,
		Col:       20,
	}
	if err := c.UpsertNote(note, []cache.Link{link}); err != nil {
		t.Fatalf("UpsertNote failed: %v", err)
	}
	fwd, err := c.GetForwardLinks(note)
	if err != nil {
		t.Fatalf("GetForwardLinks failed: %v", err)
	}
	if len(fwd) != 1 {
		t.Errorf("expected 1 forward link, got %d", len(fwd))
	} else {
		if fwd[0] != link {
			t.Errorf("forward link does not match expected values")
		}
	}
}

func TestGetBackLinks(t *testing.T) {
	c := newTestCache(t)
	source := cache.Note("back_source")
	target := cache.Note("back_target")
	link := cache.Link{
		Reference: "back_ref",
		Source:    "back_source",
		Target:    "back_target",
		Row:       5,
		Col:       15,
	}
	if err := c.UpsertNote(source, []cache.Link{link}); err != nil {
		t.Fatalf("UpsertNote failed: %v", err)
	}
	back, err := c.GetBackLinks(target)
	if err != nil {
		t.Fatalf("GetBackLinks failed: %v", err)
	}
	if len(back) != 1 {
		t.Errorf("expected 1 backlink, got %d", len(back))
	} else {
		if back[0] != link {
			t.Errorf("back link does not match expected values")
		}
	}
}

func TestGetMissingNotes(t *testing.T) {
	c := newTestCache(t)
	// Insert a note with a link referencing a missing note.
	sourceNote := cache.Note("source_note")
	link := cache.Link{
		Reference: "ref_missing",
		Source:    "source_note",
		Target:    "missing_note",
		Row:       1,
		Col:       1,
	}
	if err := c.UpsertNote(sourceNote, []cache.Link{link}); err != nil {
		t.Fatalf("UpsertNote failed: %v", err)
	}
	missing, err := c.GetMissingNotes()
	if err != nil {
		t.Fatalf("GetMissingNotes failed: %v", err)
	}
	found := false
	for _, n := range missing {
		if n == cache.Note("missing_note") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GetMissingNotes did not return the missing note")
	}
}

func TestIntegration_Filecache(t *testing.T) {
	c := newTestCache(t)

	// Step 1: Insert two notes and link them.
	noteA := cache.Note("noteA")
	noteB := cache.Note("noteB")
	link := cache.Link{
		Reference: "linkAB",
		Source:    "noteA",
		Target:    "noteB",
		Row:       5,
		Col:       10,
	}

	if err := c.UpsertNote(noteA, []cache.Link{link}); err != nil {
		t.Fatalf("UpsertNote for noteA failed: %v", err)
	}
	if err := c.UpsertNote(noteB, nil); err != nil {
		t.Fatalf("UpsertNote for noteB failed: %v", err)
	}

    // Ensure GetExistingNotes list all notes
    existingNotes, err := c.GetExistingNotes()
	if err != nil {
		t.Fatalf("GetExistingNotes failed: %v", err)
	}
    if len(existingNotes) != 2 {
		t.Errorf("expected 2 notes, got %+v", existingNotes)
    }

	// Step 2: Ensure noteB is reachable via backlinks and noteA via forward links.
	forwardLinks, err := c.GetForwardLinks(noteA)
	if err != nil {
		t.Fatalf("GetForwardLinks failed: %v", err)
	}
	if len(forwardLinks) != 1 || forwardLinks[0] != link {
		t.Errorf("expected forward link from noteA -> noteB, got %+v", forwardLinks)
	}

	backLinks, err := c.GetBackLinks(noteB)
	if err != nil {
		t.Fatalf("GetBackLinks failed: %v", err)
	}
	if len(backLinks) != 1 || backLinks[0] != link {
		t.Errorf("expected backlink from noteB <- noteA, got %+v", backLinks)
	}

	// Step 3: Delete noteB and verify that:
	// - noteB is marked missing
	// - link from noteA -> noteB is not removed
	if err := c.DeleteNote(noteB); err != nil {
		t.Fatalf("DeleteNote for noteB failed: %v", err)
	}

	existingNotes, err = c.GetExistingNotes()
	if err != nil {
		t.Fatalf("GetExistingNotes failed: %v", err)
	}
    if len(existingNotes) != 1 || existingNotes[0] != noteA {
		t.Errorf("expected noteA to remain, got %+v", existingNotes)
    }


	forwardLinks, err = c.GetForwardLinks(noteA)
	if err != nil {
		t.Fatalf("GetForwardLinks failed after deleting noteB: %v", err)
	}
	if len(forwardLinks) != 1 || forwardLinks[0] != link {
		t.Errorf("expected link from noteB -> noteA to remain, got %+v", forwardLinks)
	}

	// Step 4: Verify missing notes (since noteB was a target)
	missingNotes, err := c.GetMissingNotes()
	if err != nil {
		t.Fatalf("GetMissingNotes failed: %v", err)
	}
    existingNotes, _ = c.GetExistingNotes()
	if len(missingNotes) != 1 || missingNotes[0] != noteB {
		t.Errorf("existingNotes, got %+v", existingNotes)
		t.Errorf("expected missing noteB, got %+v", missingNotes)
	}

	// Step 5: Verify timestamp updates when modifying a note.
	time.Sleep(1 * time.Second) // Ensure a time difference
	beforeUpdate, err := c.GetLastModified(noteA)
	if err != nil {
		t.Fatalf("GetLastModified failed before update: %v", err)
	}

	// Update noteA (e.g., add a new link)
	newLink := cache.Link{
		Reference: "linkAC",
		Source:    "noteA",
		Target:    "noteC",
		Row:       2,
		Col:       3,
	}
	if err := c.UpsertNote(noteA, []cache.Link{newLink}); err != nil {
		t.Fatalf("UpsertNote failed for modifying noteA: %v", err)
	}

	afterUpdate, err := c.GetLastModified(noteA)
	if err != nil {
		t.Fatalf("GetLastModified failed after update: %v", err)
	}
	if !afterUpdate.After(beforeUpdate) {
		t.Errorf("expected last modified timestamp to update")
	}

	// Step 6: Ensure changelog events fire correctly.
    // TODO
}
