# business-crm

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
`ADMIN_EMAILS` to get in the first time; anyone else needs an admin to add them on the Team page.

## Tests

```sh
make test             # both halves; run `npm install` in frontend/ first
make test-backend     # go vet, go test -race ./...; database tests need `docker compose up -d`
make test-frontend    # lint, Vitest, build, in frontend/
```

## Database

The schema is SQL files in `internal/database/migrations/`, applied by the server at startup
([PCR 0006](docs/pcr/0006-sql-migrations.md)).

## API

| Route | Returns |
|---|---|
| `GET /api/health` | `{"service":"business-crm","status":"healthy"}`, or 503 if the database is unreachable |
| `GET /auth/login` | starts signing in: redirects to the OIDC provider |
| `GET /auth/callback` | finishes signing in, and redirects to `/`, or to `/?sign-in=not-on-team`, `domain` or `failed` |
| `POST /auth/logout` | signs this browser out: 204 |
| `GET /api/me` | the signed-in user and who they are on the team: `{sub, email, name, memberId, role, canSignOut}`; 401 if no one is signed in |
| `GET /api/deals` | every deal, with company, contact, owner, support and timeline |
| `GET /api/deals/:id` | one deal, the same way |
| `POST /api/deals` | adds a lead owned by `{ownerId}`, or by no one when it's null: `{companyId}` or a new `{company}` name, optionally `{contactId}` or a new `{contact: {name, email, role}}`, and `{dealType, semester, nextStep}` |
| `PATCH /api/deals/:id` | changes `status`, `nextStep`, `nextStepDue`, `feeSEK`, `invoiceStatus`, `contactId` or `ownerId`; `null` clears a nullable one |
| `POST /api/deals/:id/notes` | adds a quick note to the deal's timeline: `{body}` |
| `GET`, `POST /api/companies`; `PATCH`, `DELETE /api/companies/:id` | companies: `{name, website, notes}`; names are unique ignoring case |
| `GET`, `POST /api/contacts`; `PATCH`, `DELETE /api/contacts/:id` | contacts: `{companyId, name, email, role}`; emails are unique |
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
| `DEV_USER_EMAIL` | no | without an OIDC client, signs every request in as this user; local work only ([PCR 0007](docs/pcr/0007-real-sign-in-dev-user-for-local-work.md)) |
| `ADMIN_EMAILS` | no | comma-separated emails let in and made admins when they sign in ([ADR 0003](docs/adr/0003-team-members-and-roles.md)) |
| `HOST`, `PORT` | no | default all interfaces, `8000` |

## Delivery

Like every KTHAIS app ([infrastructure/docs/app-delivery.md](https://github.com/kthaisociety/infrastructure/blob/main/docs/app-delivery.md)):
PRs need a Conventional Commit title; every merge to `main` builds `ghcr.io/kthaisociety/business-crm:sha-<7>`
and deploys it to staging; merging the release PR releases `X.Y.Z` for production. Frontend and backend
share one version.

## Docs

- [`CONTEXT.md`](CONTEXT.md): vocabulary
- [`docs/adr/`](docs/adr/): decisions about one part of the code
- [`docs/pcr/`](docs/pcr/): conventions for the whole project
