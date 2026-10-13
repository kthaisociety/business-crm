-- Companies and their contacts get tables of their own, and deals point at them instead of holding the
-- company name and contact as text. One contact row per person means a fix to an email or role shows on
-- every deal, and gives email sync and Fathom one place to match a sender or attendee.
--
-- Existing deals move over: one company per name (ignoring case and surrounding spaces), and one contact
-- per email, or per name at the same company when there is no email.

-- +goose Up
CREATE TABLE companies (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL CHECK (btrim(name) <> ''),
    website    text NOT NULL DEFAULT '',
    notes      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX companies_name_key ON companies (lower(name));

CREATE TABLE contacts (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id uuid NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    name       text NOT NULL DEFAULT '',
    email      text NOT NULL DEFAULT '',
    role       text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (btrim(name) <> '' OR btrim(email) <> '')
);

-- One person per email address; contacts without an email aren't limited.
CREATE UNIQUE INDEX contacts_email_key ON contacts (lower(email)) WHERE email <> '';
CREATE INDEX contacts_company_idx ON contacts (company_id);

ALTER TABLE deals
    ADD COLUMN company_id uuid REFERENCES companies (id),
    ADD COLUMN contact_id uuid REFERENCES contacts (id);

INSERT INTO companies (name)
SELECT DISTINCT ON (lower(btrim(company))) btrim(company)
FROM deals
ORDER BY lower(btrim(company)), created_at;

UPDATE deals d
SET company_id = c.id
FROM companies c
WHERE lower(c.name) = lower(btrim(d.company));

-- Contacts with an email: the latest deal's name and role, at that deal's company.
INSERT INTO contacts (company_id, name, email, role)
SELECT DISTINCT ON (lower(btrim(contact_email))) company_id, btrim(contact_name), btrim(contact_email), btrim(contact_role)
FROM deals
WHERE btrim(contact_email) <> ''
ORDER BY lower(btrim(contact_email)), created_at DESC;

-- Contacts without an email: one per name at each company.
INSERT INTO contacts (company_id, name, role)
SELECT DISTINCT ON (company_id, lower(btrim(contact_name))) company_id, btrim(contact_name), btrim(contact_role)
FROM deals
WHERE btrim(contact_email) = '' AND btrim(contact_name) <> ''
ORDER BY company_id, lower(btrim(contact_name)), created_at DESC;

UPDATE deals d
SET contact_id = ct.id
FROM contacts ct
WHERE btrim(d.contact_email) <> '' AND lower(ct.email) = lower(btrim(d.contact_email));

UPDATE deals d
SET contact_id = ct.id
FROM contacts ct
WHERE btrim(d.contact_email) = '' AND btrim(d.contact_name) <> ''
  AND ct.email = '' AND ct.company_id = d.company_id AND lower(ct.name) = lower(btrim(d.contact_name));

ALTER TABLE deals
    ALTER COLUMN company_id SET NOT NULL,
    DROP COLUMN company,
    DROP COLUMN contact_name,
    DROP COLUMN contact_email,
    DROP COLUMN contact_role;

CREATE INDEX deals_company_idx ON deals (company_id);
CREATE INDEX deals_contact_idx ON deals (contact_id);

-- +goose Down
ALTER TABLE deals
    ADD COLUMN company       text NOT NULL DEFAULT '',
    ADD COLUMN contact_name  text NOT NULL DEFAULT '',
    ADD COLUMN contact_email text NOT NULL DEFAULT '',
    ADD COLUMN contact_role  text NOT NULL DEFAULT '';

UPDATE deals d SET company = c.name FROM companies c WHERE c.id = d.company_id;
UPDATE deals d SET contact_name = ct.name, contact_email = ct.email, contact_role = ct.role
FROM contacts ct WHERE ct.id = d.contact_id;

ALTER TABLE deals
    ADD CONSTRAINT deals_company_check CHECK (company <> ''),
    DROP COLUMN contact_id,
    DROP COLUMN company_id;

DROP TABLE contacts;
DROP TABLE companies;
