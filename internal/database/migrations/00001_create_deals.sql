-- The deal log (CONTEXT.md): one row per deal, the business-team members who own and support deals, and
-- each deal's timeline. Company and contact are fields on the deal for now; they become tables of their
-- own when the app needs to list companies or match synced emails to contacts.

-- +goose Up
CREATE TABLE members (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email      text NOT NULL UNIQUE CHECK (email <> ''),
    name       text NOT NULL CHECK (name <> ''),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE deals (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company        text NOT NULL CHECK (company <> ''),
    deal_type      text NOT NULL CHECK (deal_type IN ('Event', 'Partnership', 'Job Post')),
    event_type     text NOT NULL DEFAULT 'TBA' CHECK (event_type IN (
                       'Workshop', 'Hackathon', 'After-Work', 'Lunch Lecture', 'Networking event',
                       'Panel discussion', 'Company visit', 'Study-trip', 'Job post', 'Other', 'TBA')),
    audience       text NOT NULL DEFAULT 'Unclear' CHECK (audience IN ('Internal', 'External', 'Unclear')),
    -- HT is the autumn term, VT the spring term.
    semester       text NOT NULL CHECK (semester ~ '^(HT|VT) [0-9]{4}$'),
    status         text NOT NULL DEFAULT 'lead' CHECK (status IN (
                       'lead', 'discussing', 'agreement', 'signed', 'finished', 'canceled')),
    start_date     date,
    end_date       date CHECK (end_date IS NULL OR (start_date IS NOT NULL AND end_date >= start_date)),
    location       text NOT NULL DEFAULT '',
    owner_id       uuid REFERENCES members (id),
    contact_name   text NOT NULL DEFAULT '',
    contact_email  text NOT NULL DEFAULT '',
    contact_role   text NOT NULL DEFAULT '',
    next_step      text NOT NULL DEFAULT '',
    next_step_due  date,
    -- NULL: not agreed yet. 0: a free event.
    fee_sek        integer CHECK (fee_sek >= 0),
    invoice_status text CHECK (invoice_status IN ('not_sent', 'waiting', 'paid', 'cancelled')),
    invoice_due    date,
    drive_url      text NOT NULL DEFAULT '',
    contract_url   text NOT NULL DEFAULT '',
    write_up_url   text NOT NULL DEFAULT '',
    rating         smallint CHECK (rating BETWEEN 1 AND 5),
    notes          text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX deals_semester_idx ON deals (semester);

-- Members who help on a deal besides its owner.
CREATE TABLE deal_support (
    deal_id   uuid NOT NULL REFERENCES deals (id) ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES members (id),
    PRIMARY KEY (deal_id, member_id)
);

-- A deal's timeline. Notes come from members; email sync and Fathom will add entries with their own source
-- and the id they have there, so a re-sync can't add the same email or meeting twice.
CREATE TABLE activities (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    deal_id       uuid NOT NULL REFERENCES deals (id) ON DELETE CASCADE,
    type          text NOT NULL CHECK (type IN ('email', 'meeting', 'note')),
    source        text NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'email_sync', 'fathom')),
    external_id   text,
    author_id     uuid REFERENCES members (id),
    title         text NOT NULL CHECK (title <> ''),
    body          text NOT NULL DEFAULT '',
    bullet_points jsonb NOT NULL DEFAULT '[]',
    occurred_at   timestamptz NOT NULL DEFAULT now(),
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source, external_id)
);

CREATE INDEX activities_deal_idx ON activities (deal_id, occurred_at DESC);

-- +goose Down
DROP TABLE activities;
DROP TABLE deal_support;
DROP TABLE deals;
DROP TABLE members;
