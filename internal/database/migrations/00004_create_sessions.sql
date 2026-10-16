-- Sign-in sessions (docs/adr/0004): the browser holds a random token, this table its SHA-256, so a leaked
-- table can't be replayed as cookies. Removing a member signs them out everywhere.

-- +goose Up
CREATE TABLE sessions (
    token_hash bytea PRIMARY KEY CHECK (length(token_hash) = 32),
    member_id  uuid NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);

CREATE INDEX sessions_member_id ON sessions (member_id);

-- +goose Down
DROP TABLE sessions;
