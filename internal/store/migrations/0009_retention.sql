-- change_sequence issues the monotonic order the changed-since feed reads by,
-- per ADR-0004. Entry ids cannot serve this: an id is assigned once at
-- creation and never renumbered by a later update, so ordering by (a
-- timestamp, id) would silently skip an update or removal that lands in the
-- same second as a cursor already sitting on a higher id. Every write the
-- delta feed must report draws one value here inside its own transaction, so
-- the value it gets is exactly that write's place in commit order.
CREATE TABLE change_sequence (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    next INTEGER NOT NULL
) STRICT;
INSERT INTO change_sequence (id, next) VALUES (1, 0);

ALTER TABLE entries ADD COLUMN change_seq INTEGER NOT NULL DEFAULT 0;

-- Entries stored before this migration have no change_seq: give each one in
-- id order, so a delta reader bootstrapping against a database from before
-- this migration still sees every existing Entry once, instead of none.
UPDATE entries SET change_seq = id;
UPDATE change_sequence SET next = (SELECT COALESCE(MAX(id), 0) FROM entries);

CREATE INDEX entries_change_seq ON entries (change_seq);

-- entry_tombstones remembers an Entry retention cleanup removed, so a delta
-- reader catching up with a since cursor learns it is gone instead of never
-- hearing about it again. entry_id is not a foreign key: the Entry it names
-- no longer exists by the time a tombstone is written.
CREATE TABLE entry_tombstones (
    entry_id INTEGER PRIMARY KEY,
    deleted_at INTEGER NOT NULL,
    change_seq INTEGER NOT NULL
) STRICT;

CREATE INDEX entry_tombstones_change_seq ON entry_tombstones (change_seq);
