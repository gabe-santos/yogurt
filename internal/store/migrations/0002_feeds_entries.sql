-- Feeds are what the reader subscribes to: one row per Feed document, unique on
-- its URL so that the same Feed cannot be subscribed twice. kind is reserved for
-- the deferred newsletter source type (ADR-0001); everything subscribable today
-- is a Feed the app pulls itself.
CREATE TABLE feeds (
    id INTEGER PRIMARY KEY,
    url TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    site_url TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL DEFAULT 'pull',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
) STRICT;

-- Entries are the items a Feed carried. They are identified by whatever the
-- publisher called the item, so re-reading a Feed updates an Entry in place
-- instead of storing it again. Deleting a Feed's Entries with it is a store
-- transaction's job, not the database's, so that the order it happens in is
-- readable in one place.
CREATE TABLE entries (
    id INTEGER PRIMARY KEY,
    feed_id INTEGER NOT NULL REFERENCES feeds (id),
    guid TEXT NOT NULL,
    title TEXT NOT NULL,
    url TEXT NOT NULL,
    content TEXT NOT NULL,
    published_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    UNIQUE (feed_id, guid)
) STRICT;

-- The reading list is newest-first, over the whole collection or one Feed. id
-- breaks ties on identical timestamps so that the paging cursor is total.
CREATE INDEX entries_published_at ON entries (published_at DESC, id DESC);

CREATE INDEX entries_feed_published_at ON entries (feed_id, published_at DESC, id DESC);
