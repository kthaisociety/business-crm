# 0001. One image: Go serves the API and the frontend

Status: Accepted

## Context
The delivery pipeline (`kthaisociety/workflows`, `deployments`) builds, releases and deploys one image per
repo. The app should answer on one domain: the frontend at `/`, the API at `/api`.

## Decision
The frontend is built to static files (`frontend/dist`) and embedded into the Go binary (`frontend/embed.go`). Gin
serves `/api/*`; every other path serves a file from the build or, failing that, `index.html` for
client-side routes. The Dockerfile builds the frontend, then the Go binary, into one image.

## Consequences
- One image, one port, one version, and the shared workflows work unchanged.
- Same origin: no CORS, and the frontend needs no API URL, so nothing is baked in at build time.
- No server-side rendering. Not needed (ADR 0002).
- `go build` needs `frontend/dist` to exist: `frontend/dist/.gitkeep` is committed, and the server answers
  "frontend not built" until `npm run build` has run.
