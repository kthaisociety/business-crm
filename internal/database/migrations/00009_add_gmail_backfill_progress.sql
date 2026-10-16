-- A mailbox's first sync, saved as it goes (docs/adr/0006): the history id taken when it started, the
-- page of messages it's on, and how many of that page it has done. A sync stopped by its deadline
-- carries on from the next message. All are null (or 0) once the backfill is done.

-- +goose Up
ALTER TABLE gmail_connections
    ADD COLUMN backfill_history text,
    ADD COLUMN backfill_page text,
    ADD COLUMN backfill_done integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE gmail_connections DROP COLUMN backfill_history, DROP COLUMN backfill_page, DROP COLUMN backfill_done;
