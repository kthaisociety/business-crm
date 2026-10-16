-- A mailbox's first sync, saved as it goes (docs/adr/0006): the history id taken when it started, and
-- the next page of messages to read. A sync stopped by its deadline carries on from there next time.
-- Both are null once the backfill is done.

-- +goose Up
ALTER TABLE gmail_connections ADD COLUMN backfill_history text, ADD COLUMN backfill_page text;

-- +goose Down
ALTER TABLE gmail_connections DROP COLUMN backfill_history, DROP COLUMN backfill_page;
