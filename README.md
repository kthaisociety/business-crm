# Lucid

The KTHAIS business team's CRM: the business team's deal log as a board and a list, with follow-up nudges.
People sign in with their @kthais.com Google account, through OIDC, and only if an admin has added them to
the team ([ADR 0004](docs/adr/0004-sign-in-with-oidc.md)). Local work can use a dev user instead, with
made-up data only ([PCR 0007](docs/pcr/0007-real-sign-in-dev-user-for-local-work.md)).

One repo, one image: a Go server serves the JSON API under `/api` and the frontend (Vite + React, embedded
in the binary) on every other path.

## Running locally

Needs Go 1.26, Node 24 and Docker.

```sh
docker compose up -d                          # Postgres on 127.0.0.1:5436
cp .env.example .env
go run ./cmd/seed                             # once: made-up deals, so the board isn't empty
go run ./cmd/server                           # 127.0.0.1:8002
cd frontend && npm install && npm run dev     # http://127.0.0.1:5174, /api proxied to 8002
```

Or build the frontend once (`cd frontend && npm run build`) and open http://127.0.0.1:8002: the Go server
serves it, as in production.

Without Go, run the production image against the same Postgres instead of `go run`:

```sh
docker compose --profile app up -d --build    # app on http://127.0.0.1:8002, signed in as dev@kthais.com
```

Rebuild after backend changes (`--build` again). `npm run dev` still works against it on 5174.

### Real sign-in locally

The dev user signs every request in. To sign in for real instead, set the OIDC client in `.env` and unset
`DEV_USER_EMAIL` (see `.env.example`), and add `PUBLIC_URL/auth/callback` (e.g.
`http://127.0.0.1:5174/auth/callback`) to the client's authorized redirect URIs. Put your own email in
admin once to get in the first time (below); anyone else needs an admin to add them on the Team page.

## Tests

```sh
make test             # both halves; run `npm install` in frontend/ first
make test-backend     # go vet, go test -race ./...; database tests need `docker compose up -d`
make test-frontend    # lint, Vitest, build, in frontend/
```

## The first admin

A new environment has no admin. Run this once, from the same image, for whoever runs the team (usually the
head of business):

```sh
./lucid appoint-admin -name "Their Name" them@kthais.com        # in the image, from /app
go run ./cmd/server appoint-admin them@kthais.com               # locally
```

After that, admins hand over on the Team page: the new admin is picked there, and the old one steps down
to board or member. The command refuses while the team has an admin; `-force` is for recovery when every
admin has left without handing over ([ADR 0005](docs/adr/0005-appoint-the-first-admin-once.md)). Locally,
`go run ./cmd/seed` already makes the dev user an admin.

## Connect Gmail

Each member connects their own mailbox from the bar under the header (or the sidebar), through a second
Google consent for reading Gmail; they can disconnect any time
([ADR 0006](docs/adr/0006-email-and-meeting-touchpoints.md)). Setting it up, in the Google Cloud project
of the OAuth client:

- APIs & Services: enable the **Gmail API**
- Google Auth Platform, Data Access: add the scope `https://www.googleapis.com/auth/gmail.readonly`
  (no Google review needed: the client is Internal)
- Clients: add `PUBLIC_URL/auth/gmail/callback` to the authorized redirect URIs
- Set `TOKEN_ENCRYPTION_KEY` (`openssl rand -base64 32`), a secret like the client's

Every 5 minutes, and right after someone connects, the server syncs each connected mailbox: the first
time its last 30 days, then only what's new. Mail with a contact in From, To or Cc lands on the
contact's company's open deals (or its latest), with its subject, Gmail's snippet, sent or received, and
a link to open it in Gmail. Everything else, drafts, spam and trash are skipped, and team members'
addresses never count as contacts. If Google withdraws access, the member is asked to reconnect.

Locally, connect only your own mailbox and use your own addresses as contacts
([PCR 0007](docs/pcr/0007-real-sign-in-dev-user-for-local-work.md)).

## Fathom meetings

Finished Fathom meetings with a contact among the invitees land on the timeline of that company's open
deals (or its latest deal), with Fathom's summary, action items and a link to the recording
([ADR 0006](docs/adr/0006-email-and-meeting-touchpoints.md)). Meetings with no contact are dropped.

To connect it, one person on the team's Fathom workspace adds a webhook (Settings, API Access, Add
Webhook):

- Destination URL: `PUBLIC_URL/api/integrations/fathom`
- Triggered for: their own recordings and **shared team recordings**, so the whole team's meetings come
  through one webhook. Meetings must be shared with the team in Fathom to be seen.
- Include: summary and action items (no transcript)

Then set `FATHOM_WEBHOOK_SECRET` to the secret Fathom shows. Fathom can't reach a laptop, so locally
this is tried with test meetings through a tunnel, never real meetings with companies
([PCR 0007](docs/pcr/0007-real-sign-in-dev-user-for-local-work.md)).

## Database

The schema is SQL files in `internal/database/migrations/`, applied by the server at startup
([PCR 0006](docs/pcr/0006-sql-migrations.md)).

## API

| Route | Returns |
|---|---|
| `GET /api/health` | `{"service":"lucid","status":"healthy"}`, or 503 if the database is unreachable |
| `GET /auth/login` | starts signing in: redirects to the OIDC provider |
| `GET /auth/callback` | finishes signing in, and redirects to `/`, or to `/?sign-in=not-on-team`, `domain` or `failed` |
| `POST /auth/logout` | signs this browser out: 204 |
| `GET /auth/gmail/connect` | starts connecting the signed-in member's own Gmail; back to `/?gmail=connected`, `wrong-account`, `no-access` or `failed` |
| `GET`, `DELETE /api/gmail` | your mailbox's connection: `{connected, email, connectedAt, syncedAt, syncError}`; DELETE disconnects and revokes it |
| `POST /api/integrations/fathom` | Fathom's webhook: a finished meeting, signed with `FATHOM_WEBHOOK_SECRET` (401 otherwise); 204. Off without the secret |
| `GET /api/me` | the signed-in user and who they are on the team: `{sub, email, name, memberId, role, canSignOut}`; 401 if no one is signed in |
| `GET /api/deals` | every deal, with company, contact, owner, support and timeline |
| `GET /api/deals/:id` | one deal, the same way |
| `POST /api/deals` | adds a lead owned by `{ownerId}`, or by no one when it's null: `{companyId}` or a new `{company}` name, optionally `{contactId}` or a new `{contact: {name, email, role}}`, and `{dealType, semester, nextStep}` |
| `PATCH /api/deals/:id` | changes `status`, `nextStep`, `nextStepDue`, `feeSEK`, `invoiceStatus`, `contactId` or `ownerId`; `null` clears a nullable one |
| `POST /api/deals/:id/notes` | adds a quick note to the deal's timeline: `{body}` |
| `GET`, `POST /api/companies`; `PATCH`, `DELETE /api/companies/:id` | companies: `{name, website, notes}`; names are unique ignoring case |
| `GET`, `POST /api/contacts`; `PATCH`, `DELETE /api/contacts/:id` | contacts: `{companyId, name, email, role}`; emails are unique |
| `POST /api/members/:id/hand-over-admin` | makes the member an admin and you `{stepDownTo}`: `board` or `member`; admins only |
| `GET /api/members`; `POST`, `PATCH`, `DELETE /api/members/:id` | the business team: `{email, name, role}`, role `admin`, `board` or `member`; only admins change it (403 otherwise) |

Every `/api` route but health needs a signed-in user: a session cookie, or the dev user. A bad request gets a 400 whose `error` says what was wrong; one
that clashes with what's stored gets a 409: a taken name or email, or deleting a company or contact that deals
point at.

## Configuration

All from env, read by the Go server ([PCR 0001](docs/pcr/0001-config-from-env.md)); see `.env.example`.

| Variable | Secret | |
|---|---|---|
| `DATABASE_URL` | yes | Postgres connection string |
| `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET` | the secret | the OIDC client; setting them turns on real sign-in ([ADR 0004](docs/adr/0004-sign-in-with-oidc.md)) |
| `OIDC_ISSUER` | no | default `https://accounts.google.com`; identity-service's URL once it's live |
| `PUBLIC_URL` | no | where the app is served; the callback is `PUBLIC_URL/auth/callback`. https, except on 127.0.0.1 or localhost |
| `SIGN_IN_DOMAIN` | no | the email domain that may sign in, default `kthais.com` |
| `TOKEN_ENCRYPTION_KEY` | yes | 32 random bytes in base64; seals Gmail refresh tokens and turns on Connect Gmail ([ADR 0006](docs/adr/0006-email-and-meeting-touchpoints.md)) |
| `GMAIL_CLIENT_ID`, `GMAIL_CLIENT_SECRET` | the secret | Connect Gmail's Google client; default to the sign-in client while that's Google's |
| `FATHOM_WEBHOOK_SECRET` | yes | the team's Fathom webhook secret, `whsec_…`; turns on `/api/integrations/fathom` ([ADR 0006](docs/adr/0006-email-and-meeting-touchpoints.md)) |
| `DEV_USER_EMAIL` | no | without an OIDC client, signs every request in as this user; local work only ([PCR 0007](docs/pcr/0007-real-sign-in-dev-user-for-local-work.md)) |
| `HOST`, `PORT` | no | default all interfaces, `8000` |

## Delivery

Like every KTHAIS app ([infrastructure/docs/app-delivery.md](https://github.com/kthaisociety/infrastructure/blob/main/docs/app-delivery.md)):
PRs need a Conventional Commit title; every merge to `main` builds `ghcr.io/kthaisociety/lucid:sha-<7>`
and deploys it to staging; merging the release PR releases `X.Y.Z` for production. Frontend and backend
share one version.

| Environment | URL | Data |
|---|---|---|
| Live | `https://lucid.kthais.com` | the team's real deals; members connect Gmail; the team's Fathom webhook |
| Staging | `https://lucid-stg.kthais.com` | made-up only: the seed (`./lucid-seed`, once), test meetings, testers' own mail |

Each has its own Google OAuth client (callbacks `/auth/callback` and `/auth/gmail/callback` on its own
URL), `TOKEN_ENCRYPTION_KEY`, Fathom secret and first admin (`./lucid appoint-admin`, also on
staging: the seed makes no one an admin there, so the order doesn't matter)
([ADR 0007](docs/adr/0007-staging-holds-made-up-data.md)).

## Docs

- [`CONTEXT.md`](CONTEXT.md): vocabulary
- [`docs/adr/`](docs/adr/): decisions about one part of the code
- [`docs/pcr/`](docs/pcr/): conventions for the whole project
