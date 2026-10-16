# One image: the Go server serves the API under /api and the built frontend everywhere else
# (docs/adr/0001).

FROM node:24-alpine AS frontend
WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o lucid ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -o lucid-seed ./cmd/seed

FROM alpine:latest
RUN apk --no-cache add ca-certificates \
    && adduser -D -u 10001 app
WORKDIR /app
COPY --from=builder /app/lucid .
# Made-up deals for staging (docs/adr/0007); never run it live.
COPY --from=builder /app/lucid-seed .
ENV HOST=0.0.0.0 \
    PORT=8000
USER app
EXPOSE 8000
CMD ["./lucid"]
