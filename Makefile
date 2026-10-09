# Every check CI runs, in one place (docs/pcr/0002). Each tool finds its own tests next to the code;
# nothing here lists them. Run `npm install` in frontend/ once before `make test-frontend`.
# Database tests use the docker compose Postgres (`docker compose up -d`); CI sets its own.
export TEST_DATABASE_URL ?= postgres://crm:crm@127.0.0.1:5436/crm?sslmode=disable

.PHONY: test test-backend test-frontend

test: test-backend test-frontend

test-backend:
	go vet ./...
	go test -race ./...

test-frontend:
	cd frontend && npm run lint && npm test && npm run build
