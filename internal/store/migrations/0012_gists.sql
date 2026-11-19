-- Gists are a reader-requested condensation of an Entry's Article, or of its
-- own body when the Article cannot be reduced, written by the language model
-- the reader configured (ADR-0018). One per Entry: regenerating replaces the
-- row rather than keeping history. foreign_keys is on for every connection
-- (store.go), so deleting an Entry — by retention or otherwise — takes its
-- Gist with it with no extra statement of its own.
CREATE TABLE gists (
    entry_id INTEGER PRIMARY KEY REFERENCES entries (id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    source TEXT NOT NULL,
    model TEXT NOT NULL,
    created_at INTEGER NOT NULL
) STRICT;
