/* Notes Table */
CREATE TABLE IF NOT EXISTS notes (
    path TEXT PRIMARY KEY,
    last_modified INTEGER NOT NULL
);

/* Links Table */
CREATE TABLE IF NOT EXISTS links (
    reference TEXT NOT NULL,
    location INTEGER NOT NULL,
    source_path TEXT NOT NULL,
    target_path TEXT NOT NULL,
    FOREIGN KEY (source_path) REFERENCES notes(path) ON DELETE CASCADE,
    FOREIGN KEY (target_path) REFERENCES notes(path) ON DELETE CASCADE,
    PRIMARY KEY (source_path, target_path, reference, location)
);

/* Unresolved Table */
CREATE TABLE IF NOT EXISTS unresolved (
    reference TEXT NOT NULL,
    location INTEGER NOT NULL,
    source_path TEXT NOT NULL,
    FOREIGN KEY (source_path) REFERENCES notes(path) ON DELETE CASCADE,
    PRIMARY KEY (source_path, reference, location)
);

/* Missing Table */
CREATE TABLE IF NOT EXISTS missing (
    reference TEXT PRIMARY KEY
);

/* Changelog Table */
CREATE TABLE IF NOT EXISTS changelog (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    table_name TEXT NOT NULL,      -- e.g., "notes", "links", "unresolved"
    operation TEXT NOT NULL,       -- "INSERT", "UPDATE", "DELETE"
    timestamp INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
    data TEXT NOT NULL             -- JSON or some serialized format of the affected row
);

/* === Indexes === */
CREATE INDEX IF NOT EXISTS idx_links_source ON links(source_path);
CREATE INDEX IF NOT EXISTS idx_links_target ON links(target_path);
CREATE INDEX IF NOT EXISTS idx_unresolved_source ON unresolved(source_path);
CREATE INDEX IF NOT EXISTS idx_unresolved_reference ON unresolved(reference);

/* === Logging for Notes === */

/* Log file insertions */
CREATE TRIGGER IF NOT EXISTS trg_notes_insert
AFTER INSERT ON notes
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES ('notes', 'INSERT', json_object('path', NEW.path));
END;

/* Log file deletions */
CREATE TRIGGER IF NOT EXISTS trg_notes_delete
AFTER DELETE ON notes
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES ('notes', 'DELETE', json_object('path', OLD.path));
END;

/* === Logging for Links === */

/* Log link insertions */
CREATE TRIGGER IF NOT EXISTS trg_links_insert
AFTER INSERT ON links
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES ('links', 'INSERT', json_object(
        'source_path', NEW.source_path,
        'target_path', NEW.target_path
    ));
END;

/* Log link deletions */
CREATE TRIGGER IF NOT EXISTS trg_links_delete
AFTER DELETE ON links
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES ('links', 'DELETE', json_object(
        'source_path', OLD.source_path,
        'target_path', OLD.target_path
    ));
END;

/* === Logging for Unresolved === */

/* Log unresolved insertions */
CREATE TRIGGER IF NOT EXISTS trg_unresolved_insert
AFTER INSERT ON unresolved
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES ('unresolved', 'INSERT', json_object(
        'source_path', NEW.source_path,
        'reference', NEW.reference
    ));
END;

/* Log unresolved updates */
CREATE TRIGGER IF NOT EXISTS trg_unresolved_update
AFTER UPDATE ON unresolved
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES ('unresolved', 'UPDATE', json_object(
        'source_path', OLD.source_path,
        'old_reference', OLD.reference,
        'new_reference', NEW.reference
    ));
END;

/* Log unresolved deletions */
CREATE TRIGGER IF NOT EXISTS trg_unresolved_delete
AFTER DELETE ON unresolved
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES ('unresolved', 'DELETE', json_object(
        'source_path', OLD.source_path,
        'reference', OLD.reference
    ));
END;

/* === Logging for Missing === */

/* Log missing file insertions */
CREATE TRIGGER IF NOT EXISTS trg_missing_insert
AFTER INSERT ON missing
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES ('missing', 'INSERT', json_object(
        'reference', NEW.reference
    ));
END;

/* Log missing file deletions */
CREATE TRIGGER IF NOT EXISTS trg_missing_delete
AFTER DELETE ON missing
FOR EACH ROW
BEGIN
    INSERT INTO changelog (table_name, operation, data)
    VALUES ('missing', 'DELETE', json_object(
        'reference', OLD.reference
    ));
END;

/* === Triggers for Notes === */

/* Prevent updates on notes */
CREATE TRIGGER IF NOT EXISTS trg_notes_no_update
BEFORE UPDATE ON notes
FOR EACH ROW
WHEN NEW.path != OLD.path
BEGIN
    SELECT RAISE(FAIL, 'The path of a file cannot not be changed.');
END;

/* Move deleted links to unresolved */
CREATE TRIGGER IF NOT EXISTS trg_move_links_to_unresolved
BEFORE DELETE ON notes
FOR EACH ROW
BEGIN
    INSERT OR IGNORE INTO unresolved (reference, location, source_path)
    SELECT reference, location, source_path
    FROM links
    WHERE target_path = OLD.path;
END;

/* === Triggers for Links === */

/* Allow only the `reference` and `location` column to be updated in `links` */
CREATE TRIGGER IF NOT EXISTS trg_links_restrict_updates
BEFORE UPDATE ON links
FOR EACH ROW
WHEN NEW.source_path != OLD.source_path
   OR NEW.target_path != OLD.target_path
BEGIN
    SELECT RAISE(FAIL, 'Source and target of a link may not be changed.');
END;

/* === Triggers for Unresolved === */

/* Insert into the missing table when a new unresolved entry is added */
CREATE TRIGGER IF NOT EXISTS trg_unresolved_insert_missing
AFTER INSERT ON unresolved
FOR EACH ROW
BEGIN
    INSERT OR IGNORE INTO missing (reference)
    VALUES (NEW.reference);
END;

/* Update missing table to reflect the new reference when unresolved entry is updated */
CREATE TRIGGER IF NOT EXISTS trg_unresolved_update_missing
AFTER UPDATE ON unresolved
FOR EACH ROW
BEGIN
    -- Update the reference in the missing table
    UPDATE missing
    SET reference = NEW.reference
    WHERE reference = OLD.reference;

    -- If the old reference no longer exists in unresolved, remove it from missing
    DELETE FROM missing
    WHERE reference = OLD.reference
    AND NOT EXISTS (
        SELECT 1 FROM unresolved WHERE reference = OLD.reference
    );

    -- If the new reference is not already in missing, insert it
    INSERT OR IGNORE INTO missing (reference)
    VALUES (NEW.reference);
END;

/* Delete from the missing table when an unresolved entry is deleted */
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

/* === Triggers for Missing === */

/* Prevent direct updates on missing */
CREATE TRIGGER IF NOT EXISTS trg_missing_no_manual_update
BEFORE UPDATE ON missing
FOR EACH ROW
BEGIN
    SELECT RAISE(FAIL, 'Direct updates to the missing table are not allowed.');
END;
