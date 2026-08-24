-- Fetch state is what this application knows about a Feed's remote copy, so a
-- scheduled check can ask "has this changed?" instead of always re-reading and
-- re-parsing the whole document (ADR-0001). etag and last_modified are the
-- validators sent back as conditional-request headers; next_check_at is when
-- the schedule should next ask; last_checked_at and last_success_at let a
-- silent Feed be told apart from a failing one; last_error and
-- consecutive_failures back a Feed off the more it keeps failing.
ALTER TABLE feeds ADD COLUMN etag TEXT NOT NULL DEFAULT '';
ALTER TABLE feeds ADD COLUMN last_modified TEXT NOT NULL DEFAULT '';
ALTER TABLE feeds ADD COLUMN next_check_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE feeds ADD COLUMN last_checked_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE feeds ADD COLUMN last_success_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE feeds ADD COLUMN last_error TEXT NOT NULL DEFAULT '';
ALTER TABLE feeds ADD COLUMN consecutive_failures INTEGER NOT NULL DEFAULT 0;

-- The schedule polls whatever is due among non-suspended Feeds; the partial
-- index only ever needs to cover those.
CREATE INDEX feeds_due ON feeds (next_check_at) WHERE suspended = 0;
