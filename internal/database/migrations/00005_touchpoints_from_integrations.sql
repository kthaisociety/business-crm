-- Touchpoints from email sync and Fathom (docs/adr/0006). One email or meeting can land on several deals
-- (every open deal of the contact's company), so its external id is unique per deal, not overall. And a
-- synced touchpoint links back to where it came from: the Fathom recording, or the email in Gmail.

-- +goose Up
ALTER TABLE activities DROP CONSTRAINT activities_source_external_id_key;
ALTER TABLE activities ADD CONSTRAINT activities_deal_source_external_id_key UNIQUE (deal_id, source, external_id);
ALTER TABLE activities ADD COLUMN url text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE activities DROP COLUMN url;
ALTER TABLE activities DROP CONSTRAINT activities_deal_source_external_id_key;
-- Keeps one row per (source, external_id) so the old constraint fits again.
DELETE FROM activities a USING activities b
WHERE a.source = b.source AND a.external_id = b.external_id AND a.id > b.id;
ALTER TABLE activities ADD CONSTRAINT activities_source_external_id_key UNIQUE (source, external_id);
