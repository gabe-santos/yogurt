-- device_tokens are the non-browser client's credential, per ADR-0005: a
-- random token, stored only as a hash, presented as a bearer credential
-- anywhere the session cookie is accepted. Unlike a session, a device token
-- does not expire on its own; only revoking it (deleting the row) ends it.
CREATE TABLE device_tokens (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL,
    last_used_at INTEGER
) STRICT;
