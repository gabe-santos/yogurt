-- Articles are the publisher's own page for an Entry's link, reduced to its
-- main text by extraction and kept by URL rather than by Entry, since two
-- Entries can point at the same page. Extraction happens on demand, the first
-- time a reader asks for Reader View, so a row here means it has already been
-- fetched and reduced once and does not need fetching again.
CREATE TABLE articles (
    url TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    html TEXT NOT NULL,
    embeddable INTEGER NOT NULL,
    fetched_at INTEGER NOT NULL
) STRICT;
