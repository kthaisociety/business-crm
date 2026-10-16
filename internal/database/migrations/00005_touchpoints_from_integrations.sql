-- Touchpoints from email sync and Fathom (docs/adr/0006). One email or meeting can land on several deals
-- (every open deal of the contact's company), so its external id is unique per deal, not overall. And a
-- synced touchpoint links back to where it came from: the Fathom recording, or the email in Gmail.

-- +goose Up
ALTER TABLE activities DROP CONSTRAINT activities_source_external_id_key;
ALTER TABLE activities ADD CONSTRAINT activities_deal_source_external_id_key UNIQUE (deal_id, source, external_id);
ALTER TABLE activities ADD COLUMN url text NOT NULL DEFAULT '';

-- +goose Down
-- The old schema can hold an email or meeting on one deal only. Rather than drop it from the others, the
-- rollback refuses while any is on several deals.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM activities WHERE external_id IS NOT NULL
        GROUP BY source, external_id HAVING count(*) > 1
    ) THEN
        RAISE EXCEPTION 'can''t roll back 00005: some emails or meetings are on several deals, and the old schema keeps each on one';
    END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE activities DROP COLUMN url;
ALTER TABLE activities DROP CONSTRAINT activities_deal_source_external_id_key;
ALTER TABLE activities ADD CONSTRAINT activities_source_external_id_key UNIQUE (source, external_id);
