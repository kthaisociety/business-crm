-- A mailbox's first sync, saved as it goes (docs/adr/0006): the history id and time it started at (it
-- reads mail from before that time only, so new mail can't shift its list), the page of messages it's
-- on, and the last message it recorded. A sync stopped by its deadline carries on after that message.
-- All are null once the backfill is done.

-- +goose Up
ALTER TABLE gmail_connections
    ADD COLUMN backfill_history text,
    ADD COLUMN backfill_until timestamptz,
    ADD COLUMN backfill_page text,
    ADD COLUMN backfill_last text;

-- +goose Down
ALTER TABLE gmail_connections DROP COLUMN backfill_history, DROP COLUMN backfill_until, DROP COLUMN backfill_page, DROP COLUMN backfill_last;
