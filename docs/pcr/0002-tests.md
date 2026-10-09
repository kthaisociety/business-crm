# 0002. Tests live next to the code and run on every PR

Status: Accepted

## Convention
- Tests follow each language's convention and run in CI on every PR and every merge to `main`:
  - Go: `foo_test.go` next to `foo.go`, run with `go test ./...`.
  - Frontend: `Foo.test.tsx` next to `Foo.tsx`, run with `npm test` (Vitest) in `frontend/`.
- Every PR adds or updates tests that show its ticket's behaviour works. Writing that test first is a good
  way to pin down what "done" means.
- Tests never call real services: use `httptest.Server`, fakes, or a stubbed `fetch`.

## Why
One kind of test that always runs is easier to keep up than several kinds with their own rules.

## How to apply
`make test` runs every check: `make test-backend` (`go vet`, `go test -race ./...`) and `make test-frontend`
(`npm run lint`, `npm test` and `npm run build` in `frontend/`). CI runs the same two targets.
