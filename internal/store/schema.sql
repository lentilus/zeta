/* Notes Table */
CREATE TABLE IF NOT EXISTS notes (
    path TEXT PRIMARY KEY,
    last_modified INTEGER NOT NULL
);

/* Links Table */
CREATE TABLE IF NOT EXISTS links (
    reference TEXT NOT NULL,
    row INTEGER NOT NULL,
    col INTEGER NOT NULL,
    source_path TEXT NOT NULL,
    target_path TEXT NOT NULL,
    FOREIGN KEY (source_path) REFERENCES notes(path) ON DELETE CASCADE,
    FOREIGN KEY (target_path) REFERENCES notes(path) ON DELETE CASCADE,
    UNIQUE (source_path, reference, row, col)
);

/* Unresolved Table */
CREATE TABLE IF NOT EXISTS unresolved (
    reference TEXT NOT NULL,
    row INTEGER NOT NULL,
    col INTEGER NOT NULL,
    source_path TEXT NOT NULL,
    FOREIGN KEY (source_path) REFERENCES notes(path) ON DELETE CASCADE,
    UNIQUE (source_path, reference, row, col)
);

/* Missing Table */
CREATE TABLE IF NOT EXISTS missing (
    reference TEXT PRIMARY KEY
);

/* Changelog Table */
CREATE TABLE IF NOT EXISTS changelog (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name TEXT NOT NULL,      -- e.g., "notes", "links", "unresolved"
    operation TEXT NOT NULL,       -- "INSERT", "DELETE"
    data TEXT NOT NULL             -- JSON or some serialized format of the affected row
    timestamp INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
);

/* === Indexes === */
CREATE INDEX IF NOT EXISTS idx_links_source ON links(source_path);
CREATE INDEX IF NOT EXISTS idx_links_target ON links(target_path);
CREATE INDEX IF NOT EXISTS idx_unresolved_source ON unresolved(source_path);
CREATE INDEX IF NOT EXISTS idx_unresolved_reference ON unresolved(reference);

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

/* === Logging for Unresolved === */
/* Log unresolved insertions */
CREATE TRIGGER IF NOT EXISTS trg_unresolved_insert
AFTER INSERT ON unresolved
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES (
        'unresolved',
        'INSERT',
        json_object(
            'source_path', NEW.source_path,
            'reference', NEW.reference,
        )
    );
END;

/* Log unresolved deletions */
CREATE TRIGGER IF NOT EXISTS trg_unresolved_delete
AFTER DELETE ON unresolved
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES (
        'unresolved',
        'DELETE',
        json_object(
            'source_path', OLD.source_path,
            'reference', OLD.reference,
        )
    );
END;

/* Disallow any updates on unresolved */
CREATE TRIGGER IF NOT EXISTS trg_unresolved_no_update
BEFORE UPDATE ON unresolved
FOR EACH ROW
BEGIN
    SELECT RAISE(FAIL, 'Updates on unresolved are not allowed.');
END;

/* === Logging for Missing === */
/* Log missing insertions */
CREATE TRIGGER IF NOT EXISTS trg_missing_insert
AFTER INSERT ON missing
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES ('missing', 'INSERT', json_object('reference', NEW.reference));
END;

/* Log missing deletions */
CREATE TRIGGER IF NOT EXISTS trg_missing_delete
AFTER DELETE ON missing
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES ('missing', 'DELETE', json_object('reference', OLD.reference));
END;

/* Disallow any updates on missing */
CREATE TRIGGER IF NOT EXISTS trg_missing_no_update
BEFORE UPDATE ON missing
FOR EACH ROW
BEGIN
    SELECT RAISE(FAIL, 'Updates on missing are not allowed.');
END;

/* === Additional Triggers === */
/* When a note is deleted, move any links pointing to it into unresolved */
CREATE TRIGGER IF NOT EXISTS trg_move_links_to_unresolved
BEFORE DELETE ON notes
FOR EACH ROW
BEGIN
    INSERT OR IGNORE INTO unresolved (reference, location, source_path)
    SELECT reference, location, source_path
    FROM links
    WHERE target_path = OLD.path;
END;

/* When an unresolved row is inserted, ensure the missing table reflects the new reference */
CREATE TRIGGER IF NOT EXISTS trg_unresolved_insert_missing
AFTER INSERT ON unresolved
FOR EACH ROW
BEGIN
    INSERT OR IGNORE INTO missing (reference)
    VALUES (NEW.reference);
END;

/* When an unresolved row is deleted, remove the reference from missing if it no longer exists */
CREATE TRIGGER IF NOT EXISTS trg_unresolved_delete_missing
AFTER DELETE ON unresolved
FOR EACH ROW
BEGIN
    DELETE FROM missing
    WHERE reference = OLD.reference
      AND NOT EXISTS (
          SELECT 1 FROM unresolved WHERE reference = OLD.reference
      );
END;
