package cache_test

// TODO: add as tests
// The graph topology is independend of the order of added links / notes
// The graph topology is independend of the order of deleted links / notes
// Adding a note <stuff> deleting a note should be equivalent to <stuff> (even if invalid operation requests?)
// deleting a note, <stuff> ,adding a note should be equivalent to <stuff> (even if invalid operation requests?)

import (
	"testing"
	c "zeta/internal/cache"
)

func FuzzCache(f *testing.F) {
	f.Add([]byte("seed1"))
	f.Add([]byte("fuzz!"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			t.Skip("Skipping empty input")
		}

		pc := c.NewHybridCache()
		pos := 0
		numOps := int(data[0])%20 + 1
		pos++

		for i := 0; i < numOps && pos < len(data); i++ {
			if pos >= len(data) {
				break
			}
			opType := data[pos] % 7
			pos++

			notePath, nPos := genString(data, pos, 1, 5)
			if nPos > len(data) {
				break
			}
			pos = nPos

			note := c.Note{Path: c.Path(notePath)}

			if pos >= len(data) {
				break
			}
			numLinks := int(data[pos]) % 4
			pos++

			links := make([]c.Link, 0, numLinks)
			for j := 0; j < numLinks && pos < len(data); j++ {
				tgtPath, nPos := genString(data, pos, 1, 5)
				if nPos > len(data) {
					break
				}
				pos = nPos

				var row, col uint
				var ref string

				if pos < len(data) {
					row = uint(data[pos])
					pos++
				}
				if pos < len(data) {
					col = uint(data[pos])
					pos++
				}
				ref, nPos = genString(data, pos, 1, 5)
				if nPos > len(data) {
					break
				}
				pos = nPos

				links = append(links, c.Link{
					Row: row,
					Col: col,
					Ref: ref,
					Src: note.Path,
					Tgt: c.Path(tgtPath),
				})
			}

			switch opType {
			case 0:
				if err := pc.Upsert(note, links); err != nil {
					t.Logf("Upsert error for note %s: %v", note.Path, err)
				}
			case 1:
				if err := pc.UpsertTmp(note, links); err != nil {
					t.Logf("UpsertTmp error for note %s: %v", note.Path, err)
				}
			case 2:
				if err := pc.Delete(note.Path); err != nil {
					t.Logf("Delete error for note %s: %v", note.Path, err)
				}
			case 3:
				if err := pc.DeleteTmp(note.Path); err != nil {
					t.Logf("DeleteTmp error for note %s: %v", note.Path, err)
				}
			case 4:
				if _, err := pc.Paths(); err != nil {
					t.Logf("Paths error: %v", err)
				}
			case 5:
				if _, err := pc.ForwardLinks(note.Path); err != nil {
					t.Logf("ForwardLinks error for note %s: %v", note.Path, err)
				}
			case 6:
				if _, err := pc.BackLinks(note.Path); err != nil {
					t.Logf("BackLinks error for note %s: %v", note.Path, err)
				}
			}
		}
	})
}

func genString(data []byte, pos int, minLen, maxLen int) (string, int) {
	if pos >= len(data) {
		return "", pos
	}
	length := int(data[pos])%(maxLen-minLen+1) + minLen
	pos++
	if pos >= len(data) {
		return "", pos
	}
	if pos+length > len(data) {
		length = len(data) - pos
	}
	return string(data[pos : pos+length]), pos + length
}
