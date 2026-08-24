-- Groups are named sets of Feeds used to scope reading to one part of the
-- collection. A Feed belongs to exactly one Group, and Groups do not nest.
-- The default Group holds anything the reader has not sorted, so a Feed is
-- never unreachable; it is seeded here and cannot be deleted (enforced in the
-- store, not by a CHECK, so the refusal can carry a reason).
CREATE TABLE groups (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    is_default INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

INSERT INTO groups (id, name, is_default, created_at, updated_at)
VALUES (1, 'Unsorted', 1, unixepoch(), unixepoch());

-- Every existing and future Feed belongs to exactly one Group; the default
-- backfills every Feed subscribed before Groups existed. suspended lets a
-- noisy Feed go quiet without deleting its history.
ALTER TABLE feeds ADD COLUMN group_id INTEGER NOT NULL DEFAULT 1;
ALTER TABLE feeds ADD COLUMN suspended INTEGER NOT NULL DEFAULT 0;

CREATE INDEX feeds_group_id ON feeds (group_id);
