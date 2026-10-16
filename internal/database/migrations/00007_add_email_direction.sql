-- Which way a synced email went (docs/adr/0006): sent by someone on the team, or received from outside.
-- Follow-up nudges use it to say whose turn it is. Empty for notes and meetings.

-- +goose Up
ALTER TABLE activities ADD COLUMN direction text NOT NULL DEFAULT '' CHECK (direction IN ('', 'sent', 'received'));

-- +goose Down
ALTER TABLE activities DROP COLUMN direction;
