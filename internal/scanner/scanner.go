// scanner is used to scan a directory for notes.
package scanner

import (
    "io/fs"
    "log"
    "os"
    "path/filepath"
    "strings"
)

// Scan walks the entire subtree under root.  Any file or directory
// whose name begins with “.” is skipped entirely.  For each remaining
// file, we apply your skip() predicate, and if that returns false
// we read the file and invoke callback(relPath, contents).
func Scan(
    root string,
    skip func(relPath string, info fs.FileInfo) bool,
    callback func(relPath string, document []byte),
) {
    fileCh := make(chan string, 100)

    // worker goroutine
    go func() {
        for rel := range fileCh {
            full := filepath.Join(root, rel)
            data, err := os.ReadFile(full)
            if err != nil {
                log.Println("scanner: read error:", full, err)
                continue
            }
            callback(rel, data)
        }
    }()

    log.Printf("scanner: starting WalkDir at %q", root)
    err := filepath.WalkDir(root, func(fullPath string, d fs.DirEntry, err error) error {
        if err != nil {
            log.Println("scanner: walk error:", err)
            return nil
        }

        // get path relative to root
        rel, err := filepath.Rel(root, fullPath)
        if err != nil {
            return nil
        }
        if rel == "." {
            // the root itself
            return nil
        }

        // log every directory we actually enter
        if d.IsDir() {
            log.Printf("scanner: descending into %q", rel)
        }

        // skip any path component starting with "."
        for _, part := range strings.Split(rel, string(os.PathSeparator)) {
            if strings.HasPrefix(part, ".") {
                if d.IsDir() {
                    return filepath.SkipDir
                }
                return nil
            }
        }

        // if it’s a directory, keep recursing
        if d.IsDir() {
            return nil
        }

        // now it’s a file: apply your skip()
        info, err := d.Info()
        if err != nil {
            return nil
        }
        if skip(rel, info) {
            return nil
        }

        // finally, enqueue for reading
        fileCh <- rel
        return nil
    })
    if err != nil {
        log.Println("scanner: WalkDir finished with error:", err)
    }

    // tell the worker we’re done
    close(fileCh)
}
