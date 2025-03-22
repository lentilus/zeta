# Cache

- The cache is split up into two cache layers:
  - The Tmp-Cache holds the state of notes opened in the editor but not yet saved.
  - The Pst-Cache holds the state of notes as they are saved on disk.
  This means, that the Pst-Cache tends to hold the bulk of the information and
  The Tmp-Cache only serves to overlay the most recent information from notes
  that are being edited in real-time.
  Generally both caches are maintained in way that avoids conflicts in their
  note-graph. Links to notes that do not exist in the respective cache layer
  induce the addition of a note with the flag `missing`

Here is a small example:

The State on disk is like this

```
Note A:
---
Has a link to [[Note B]]

Note B:
--
Has a link to [[Note C]]
```

The Pst-Cache would the hold Note A, Note B, and Note C (`missing`), since
there is a Link to Note B

in Memory the state is
```
Note A:
---
Has no links.

Note C:
---
Has a link to [[Note B]] and [[Note D]]
```

The Tmp-Cache would then hold Note A, Note C, Note B (`missing`), and Note D (`missing`)

In combined state, presented to the rest of the applicatio would be

Notes:
Note A, Note B, Note C, Note D (`missing`)
Links:
- B -> C
- C -> B
- C -> D

Note that there is no Link from A -> B since the Note A in memory has priority.


