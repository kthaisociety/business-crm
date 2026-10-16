# 0007. Staging holds made-up data; only live holds the team's

Status: Accepted

## Context
The CRM runs in two places: live at `lucid.kthais.com`, and staging at `lucid-stg.kthais.com`, where
every merge to `main` deploys first. ADR 0006 planned for no staging. Both sign in for real (ADR 0004),
so PCR 0007 would let both hold real data. But a copy of the team's deals, and of contacts' emails and
meeting notes, in a second place doubles what can leak, and a bug being tried on staging could spoil
real records.

## Decision
- **Live** holds the team's real deals, companies and contacts, and is where members connect Gmail and
  the team's Fathom webhook points.
- **Staging** holds made-up data only:
  - Its deals come from the seed, run once in the image: `./business-crm-seed`.
  - The team's Fathom webhook never points at it. Someone trying Fathom there adds their own webhook for
    their own recordings only, and holds test meetings.
  - Only people trying Gmail connect there, with contacts on their own addresses, as locally.
- Each environment has its own Google OAuth client, with only its own callbacks, and its own
  `TOKEN_ENCRYPTION_KEY`, Fathom secret and first admin (ADR 0005).
- This replaces ADR 0006's "there's no staging": what's tried before going live is tried on staging, the
  same way as locally.

## Consequences
- Trying a change on staging never touches the team's data, and staging's database can be wiped.
- Staging shows the app as it will be, but not with the team's real volume of deals and mail.
- The seed ships in the image next to the server.
