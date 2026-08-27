-- Read is set once the reader has seen an Entry's contents: always settable and
-- unsettable by hand, and by default set when the Entry is opened (client-side,
-- gated by the mark_on_open setting). SaveEntries never touches it, so a Feed
-- re-publishing the same item does not un-read it.
ALTER TABLE entries ADD COLUMN read INTEGER NOT NULL DEFAULT 0;

-- Unread Only is a third reading-list shape alongside the two 0002 indexes:
-- newest-first over just the unread Entries, so narrowing a Collection to them
-- does not fall back to a full scan of entries_published_at.
CREATE INDEX entries_unread_published_at ON entries (published_at DESC, id DESC) WHERE read = 0;

-- Settings are the reader's own preferences: one row per key, so that a new
-- preference is a row rather than a new column and a migration.
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;
