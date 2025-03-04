/* Notes Table */
CREATE TABLE IF NOT EXISTS notes (
    path TEXT PRIMARY KEY,
    last_modified INTEGER NOT NULL,
    exists INTEGER NOT NULL DEFAULT 1
);

/* Links Table */
CREATE TABLE IF NOT EXISTS links (
    reference TEXT NOT NULL,
    row INTEGER NOT NULL,
    col INTEGER NOT NULL,
    source_path TEXT NOT NULL,
    target_path TEXT NOT NULL,
    FOREIGN KEY (source_path) REFERENCES notes(path) ON DELETE CASCADE,
    FOREIGN KEY (target_path) REFERENCES notes(path) ON DELETE RESTRICT,
    UNIQUE (source_path, reference, row, col)
);

/* Changelog Table */
CREATE TABLE IF NOT EXISTS changelog (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name TEXT NOT NULL,      -- e.g., "notes", "links"
    operation TEXT NOT NULL,       -- "INSERT", "UPDATE", "DELETE"
    data TEXT NOT NULL,            -- JSON or some serialized format of the affected row
    timestamp INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);

/* === Indexes === */
CREATE INDEX IF NOT EXISTS idx_links_source ON links(source_path);
CREATE INDEX IF NOT EXISTS idx_links_target ON links(target_path);

/* === Trigger: Ensure Link Target Exists === */
/* Before inserting a link, automatically create a target note with exists=0 if it doesn't exist */
CREATE TRIGGER IF NOT EXISTS trg_links_insert_ensure_target
BEFORE INSERT ON links
FOR EACH ROW
BEGIN
    INSERT OR IGNORE INTO notes (path, last_modified, exists)
    VALUES (NEW.target_path, strftime('%s','now'), 0);
END;

/* === Trigger: Remove Target Note if Orphaned === */
/* After deleting a link, if the target note was auto-created (exists=0) and has no other backlinks, delete it */
CREATE TRIGGER IF NOT EXISTS trg_links_delete_remove_target
AFTER DELETE ON links
FOR EACH ROW
BEGIN
    DELETE FROM notes
    WHERE path = OLD.target_path
      AND exists = 0
      AND NOT EXISTS (
          SELECT 1 FROM links WHERE target_path = OLD.target_path
      );
END;

/* === Logging for Notes === */
/* Log note insertions */
CREATE TRIGGER IF NOT EXISTS trg_notes_insert
AFTER INSERT ON notes
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES ('notes', 'INSERT', json_object('path', NEW.path));
END;

/* Log note deletions */
CREATE TRIGGER IF NOT EXISTS trg_notes_delete
AFTER DELETE ON notes
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES ('notes', 'DELETE', json_object('path', OLD.path));
END;

/* Prevent illegal updates on notes:
   Only allow an update if the primary key (path) remains unchanged. */
CREATE TRIGGER IF NOT EXISTS trg_notes_no_illegal_update
BEFORE UPDATE ON notes
FOR EACH ROW
WHEN NEW.path != OLD.path
BEGIN
    SELECT RAISE(FAIL, 'The path of a file cannot be changed.');
END;

/* === Logging for Links === */
/* Log link insertions */
CREATE TRIGGER IF NOT EXISTS trg_links_insert
AFTER INSERT ON links
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES (
        'links',
        'INSERT',
        json_object(
            'source_path', NEW.source_path,
            'target_path', NEW.target_path,
            'reference', NEW.reference,
            'row', NEW.row,
            'col', NEW.col
        )
    );
END;

/* Log link deletions */
CREATE TRIGGER IF NOT EXISTS trg_links_delete
AFTER DELETE ON links
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES (
        'links',
        'DELETE',
        json_object(
            'source_path', OLD.source_path,
            'target_path', OLD.target_path,
            'reference', OLD.reference,
            'row', OLD.row,
            'col', OLD.col
        )
    );
END;

/* Disallow any updates on links */
CREATE TRIGGER IF NOT EXISTS trg_links_no_update
BEFORE UPDATE ON links
FOR EACH ROW
BEGIN
    SELECT RAISE(FAIL, 'Updates on links are not allowed.');
END;
