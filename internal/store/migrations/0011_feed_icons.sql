-- A Feed Icon is stored as a blob on the Feed row rather than as a file in a
-- new directory, so the data directory stays a single database file (ADR-0008).
-- icon_stored_at is absent (0) until an icon is first stored, so the API can
-- tell a Feed with no icon apart from one that has not been checked yet, and
-- doubles as a cache-busting version for the serving endpoint. icon_checked_at
-- gates both outcomes of a check, successful or not, so a site with no usable
-- icon is re-probed about once a month instead of on every poll.
ALTER TABLE feeds ADD COLUMN icon_data BLOB;
ALTER TABLE feeds ADD COLUMN icon_media_type TEXT NOT NULL DEFAULT '';
ALTER TABLE feeds ADD COLUMN icon_stored_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE feeds ADD COLUMN icon_checked_at INTEGER NOT NULL DEFAULT 0;
