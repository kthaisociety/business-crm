-- Every member has a role (docs/adr/0003): admin manages the team, board and member work deals.
-- Member emails are stored in lower case, so the same person signing in as Alva@ or alva@, or added by an
-- admin before signing in, is one member.

-- +goose Up
ALTER TABLE members
    ADD COLUMN role text NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'board', 'member'));

-- Members whose emails differ only in case are one person: keep the earliest and move the others' deals,
-- support and notes to them, so lowercasing can't collide.
CREATE TEMP TABLE member_merges ON COMMIT DROP AS
SELECT m.id AS old_id, keep.id AS keep_id
FROM members m
JOIN LATERAL (
    SELECT k.id FROM members k WHERE lower(k.email) = lower(m.email) ORDER BY k.created_at, k.id LIMIT 1
) keep ON keep.id <> m.id;

UPDATE deals SET owner_id = mm.keep_id FROM member_merges mm WHERE deals.owner_id = mm.old_id;
INSERT INTO deal_support (deal_id, member_id)
SELECT ds.deal_id, mm.keep_id FROM deal_support ds JOIN member_merges mm ON ds.member_id = mm.old_id
ON CONFLICT DO NOTHING;
DELETE FROM deal_support ds USING member_merges mm WHERE ds.member_id = mm.old_id;
UPDATE activities SET author_id = mm.keep_id FROM member_merges mm WHERE activities.author_id = mm.old_id;
DELETE FROM members m USING member_merges mm WHERE m.id = mm.old_id;

UPDATE members SET email = lower(email);
ALTER TABLE members ADD CONSTRAINT members_email_lower CHECK (email = lower(email));

-- +goose Down
ALTER TABLE members DROP CONSTRAINT members_email_lower;
ALTER TABLE members DROP COLUMN role;
