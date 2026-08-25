-- Full-text search over what a Feed itself supplied: an Entry's title and
-- body content, reduced to plain text — search should match the words a
-- publisher wrote, not their HTML markup. Per the MVP PRD, extracted
-- Articles are not indexed — extraction is on-demand, which would make
-- coverage silently uneven.
--
-- entries_fts stores its own plain-text copy rather than reading entries
-- directly (an external-content table), because reducing HTML to plain text
-- is application logic, not something a SQL trigger can do; the Store keeps
-- it in step from Go, in the same transaction as every write that changes an
-- Entry's title or content, or removes it (internal/store/search.go).
CREATE VIRTUAL TABLE entries_fts USING fts5(
    title,
    content,
    tokenize = 'unicode61'
);
