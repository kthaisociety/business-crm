-- Members' connected Gmail mailboxes (docs/adr/0006). The refresh token is sealed with
-- TOKEN_ENCRYPTION_KEY, so the table alone can't read anyone's mail. Removing a member removes it.

-- +goose Up
CREATE TABLE gmail_connections (
    member_id     uuid PRIMARY KEY REFERENCES members (id) ON DELETE CASCADE,
    email         text NOT NULL CHECK (email = lower(email)),
    refresh_token bytea NOT NULL,
    -- Where sync carries on from: Gmail's history id, null until the first sync.
    history_id    text,
    connected_at  timestamptz NOT NULL DEFAULT now(),
    synced_at     timestamptz,
    -- The last sync's error, shown to the member, or '' when it worked.
    sync_error    text NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE gmail_connections;
