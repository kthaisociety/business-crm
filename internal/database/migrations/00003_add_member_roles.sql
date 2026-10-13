-- Every member has a role (docs/adr/0003): admin manages the team, board and member work deals.
-- Member emails are stored in lower case, so the same person signing in as Alva@ or alva@, or added by an
-- admin before signing in, is one member.

-- +goose Up
ALTER TABLE members
    ADD COLUMN role text NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'board', 'member'));

UPDATE members SET email = lower(email);
ALTER TABLE members ADD CONSTRAINT members_email_lower CHECK (email = lower(email));

-- +goose Down
ALTER TABLE members DROP CONSTRAINT members_email_lower;
ALTER TABLE members DROP COLUMN role;
