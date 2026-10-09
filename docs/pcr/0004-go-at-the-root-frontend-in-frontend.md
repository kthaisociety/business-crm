# 0004. The Go module is the repo root; the frontend lives in `frontend/`

Status: Accepted

## Convention
- `go.mod`, `cmd/` and `internal/` sit at the repo root. There is no `backend/` folder.
- The frontend is one folder, `frontend/`, with its own `package.json`. It holds one Go file, `embed.go`,
  which embeds `frontend/dist` into the binary (ADR 0001).
- `go.mod` ignores `./frontend/node_modules`, so `./...` never walks into npm packages.
- Every KTHAIS Go repo follows this layout, so a frontend can be added to any of them without moving code.

## Why
The alternative was two root folders, `backend/` and `frontend/`. We rejected it because:
- `go:embed` only reaches files in or below the directory of the file that uses it. With `backend/` and
  `frontend/` as siblings, the Go code can't embed the build without a copy step or a Go file outside
  `backend/`.
- With `go.mod` at the root, `go run ./cmd/server`, `go test ./...` and the Dockerfile's `COPY . .` work
  from the repo root, and the module path is `github.com/kthaisociety/<repo>` with no `/backend` suffix.
- The two halves ship as one image with one version (ADR 0001). Two root folders would suggest they can be
  built and released apart, and they can't.
- Backend-only repos (luma-service, identity-service) already look like this. Adding a frontend to one of
  them is adding a folder, not restructuring the repo.

What it costs:
- The root is the Go module, so the frontend reads as a subfolder of the backend rather than an equal
  half. New members looking for "the backend" find it spread over `cmd/`, `internal/` and root files.
- One Go file lives inside the frontend folder, and `frontend/` is also a Go package.
- A CODEOWNERS rule can't name "the backend" with one path; it needs `cmd/`, `internal/`, `go.mod` and
  `go.sum`.

## How to apply
Put Go code under `cmd/` and `internal/`, and everything the browser runs under `frontend/`. Run npm
commands in `frontend/` and Go commands at the root; `make test` runs both (PCR 0002).
