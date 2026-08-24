-- Sessions are the browser's credential: a random token, stored only as a hash
-- so that a database copy cannot be replayed against a running instance.
CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
) STRICT;

CREATE INDEX sessions_expires_at ON sessions (expires_at);
