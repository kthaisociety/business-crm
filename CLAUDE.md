# CLAUDE.md

business-crm: the KTHAIS business team's CRM: the deal log as a board and a list. Sign-in is simulated
until identity-service is live (PCR 0005). One repo, one image: a Go server (Gin, GORM, Postgres) serves
the API under `/api` and an embedded Vite + React frontend everywhere else.

## Commands

- Local Postgres: `docker compose up -d`
- Config: `cp .env.example .env`
- Made-up data for local dev: `go run ./cmd/seed` (once, on an empty database)
- Backend: `go run ./cmd/server` (on `127.0.0.1:8002`)
- Frontend dev: `cd frontend && npm install && npm run dev` (Vite on 5174, proxies `/api` to 8002)
- All checks: `make test` (needs `docker compose up -d`, and `npm install` in `frontend/` once)
- Image: `docker build -t business-crm .`

## Layout

- `cmd/server`: wiring and startup; `cmd/seed`: made-up deals for local dev
- `internal/config`: env, validated at boot
- `internal/auth`: who is signed in; the dev user until identity-service (PCR 0005)
- `internal/deals`: the deal log: GORM models, validation and queries
- `internal/handlers`: `/api` routes, and the frontend for every other path
- `internal/database`: Postgres connection and migrations (`migrations/*.sql`, applied at startup)
- `frontend/`: the frontend (Vite, React, TypeScript, Tailwind); `frontend/embed.go` embeds `frontend/dist`

## Rules

- Read `CONTEXT.md` for vocabulary, `docs/adr/` for decisions and `docs/pcr/` for conventions before
  changing a part they cover. Record a new decision as a new ADR or PCR, never by editing an accepted one.
- Sign-in is simulated until identity-service is live (PCR 0005): handlers get the user from
  `auth.Current`, and the app holds no real personal data; seed and test data are made up.
- No `VITE_*` or other build-time config: the frontend calls `/api` on its own origin (PCR 0001).
- Use npm in `frontend/`; don't add another lockfile.
- Schema changes are new SQL files in `internal/database/migrations/`, never `AutoMigrate` (PCR 0006).
- PR titles and commits are Conventional Commits (`feat:`, `fix:`, `docs:`, …): they drive releases.
