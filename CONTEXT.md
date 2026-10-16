# Vocabulary

Words as this project uses them.

- **Business CRM**: this app. The business team's CRM, for KTHAIS's relations with companies.
- **Business team**: the KTHAIS team that runs company relations and partnerships.
- **User**: the person signed in.
- **Sign-in**: with an @kthais.com Google account through OIDC, for people an admin has added to the team;
  it lasts 7 days per browser ([ADR 0004](docs/adr/0004-sign-in-with-oidc.md)).
- **Dev user**: for local work without an OIDC client, the user every request is signed in as
  ([PCR 0007](docs/pcr/0007-real-sign-in-dev-user-for-local-work.md)).
- **Deal**: one piece of work with a company in one semester: an event, a partnership or a job post. A
  company can have many deals (one per semester, or several at once). Replaces a row in the team's deal
  log spreadsheet.
- **Status**: where a deal is: Lead → Discussing → Agreement → Signed → Finished, or Canceled. The board
  has one column per status.
- **Semester**: HT (autumn) or VT (spring) plus the year, e.g. "HT 2026". The app shows one semester at a
  time.
- **Company**: an organisation the team works with. It has deals and contacts, and its name is unique.
- **Contact**: a person at a company, identified by their email. A deal's contact always works at the
  deal's company, and email sync and Fathom will match senders and attendees to contacts.
- **Team member**: someone on the business team, with a role: **admin** (also manages the team),
  **board** or **member** ([ADR 0003](docs/adr/0003-team-members-and-roles.md)). A new environment's
  first admin is **appointed** once with a command; after that an admin **hands over** to the next head of
  business on the Team page ([ADR 0005](docs/adr/0005-appoint-the-first-admin-once.md)).
- **Owner** and **Support**: the team member who runs a deal, and the members who help. A lead can have no
  owner yet; anyone can **claim** it (assign it to themselves) or assign it to someone else.
- **Next step**: the one thing to do next on a deal, with an optional due date.
- **Touchpoint**: an entry on a deal's timeline: an email, a meeting or a note. Email sync (each
  member's connected Gmail) and Fathom meeting notes add touchpoints on their own
  ([ADR 0006](docs/adr/0006-email-and-meeting-touchpoints.md)).
- **Follow-up nudge**: the one warning an open deal shows when it needs attention, the first that applies:
  its next step is overdue; the company's email has waited 2 days for our reply ("your turn"); our email
  has waited 5 days for theirs ("no reply"); or, with a note, meeting or nothing last, no touchpoint for
  4 days ([ADR 0006](docs/adr/0006-email-and-meeting-touchpoints.md)).
- **Fee** and **Invoice**: what the company pays (0 for a free event), and whether that has been invoiced
  and paid.
