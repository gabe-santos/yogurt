-- Starred marks an Entry the reader wants to keep. Archived marks an Entry the
-- reader is finished with; Archived Entries are only listed in the Archive
-- view. Feed refreshes never touch either reader-owned state.
ALTER TABLE entries ADD COLUMN starred INTEGER NOT NULL DEFAULT 0;
ALTER TABLE entries ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;

CREATE INDEX entries_starred_published_at
    ON entries (published_at DESC, id DESC)
    WHERE starred = 1 AND archived = 0;

CREATE INDEX entries_archived_published_at
    ON entries (published_at DESC, id DESC)
    WHERE archived = 1;
