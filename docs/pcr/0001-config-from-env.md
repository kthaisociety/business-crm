# 0001. All configuration comes from the environment, at runtime

Status: Accepted

## Convention
Every setting is an environment variable of the Go server, read once in `internal/config` at boot. Required
ones are checked in `Config.Validate`, so a missing one stops the app at startup. The frontend has no
configuration: it calls its own origin (`/api`), and no `VITE_*` variables are used. Nothing secret is ever
committed or baked into the image. `.env` is for local dev only and is git-ignored; `.env.example` lists
every variable.

## Why
The same image runs in staging and production with different env, set by `kthaisociety/deployments`.
Build-time variables would differ between the two. Secrets live in OpenBao.

## How to apply
A new setting means a field in `Config`, a line in `.env.example` and, when deploying, a line in the
project's `project.yaml` in `deployments`. If the frontend needs a value, serve it from an `/api` endpoint.
