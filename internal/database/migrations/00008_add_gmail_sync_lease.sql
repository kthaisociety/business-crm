-- Which replica is syncing a mailbox (docs/adr/0006): it takes a lease until a time, and nobody waits
-- on it. A row lock held for the whole sync would make removing a member wait on Gmail.

-- +goose Up
ALTER TABLE gmail_connections ADD COLUMN lease_until timestamptz;

-- +goose Down
ALTER TABLE gmail_connections DROP COLUMN lease_until;
