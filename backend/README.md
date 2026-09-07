# findtutor — backend

Go + gin JSON API for the findtutor language-tutoring marketplace (Uzbekistan).
Split into two binaries: the HTTP API (`cmd/api`) and the background worker
(`cmd/worker`).

## Stack

| Concern       | Choice |
| ------------- | ------ |
| HTTP          | gin |
| DB access     | sqlc + pgx/v5 (pgxpool) |
| Migrations    | golang-migrate, run as a library via `cmd/migrate` |
| Background jobs | asynq (Redis), separate `cmd/worker` binary |
| Config        | env vars, `.env` in dev (godotenv) |
| Contract      | `../openapi.yaml` (hand-written, source of truth) |

## Layout

```
cmd/
  api/        HTTP server
  worker/     asynq job processor
  migrate/    golang-migrate runner (up / down / version / force)
  seed/       loads the demo teacher catalog (mirrors the frontend mock data)
internal/
  config/     env-based configuration
  db/         pgxpool connect + sqlc-generated code (db/sqlc)
  db/queries/ hand-written SQL (input to sqlc)
  web/        shared HTTP primitives (error envelope, response helpers)
  httpapi/    router assembly, middleware, health check
  teachers/   first domain module — teacher.go, service.go, repository_postgres.go,
              handler.go, dto.go
  availability/  teacher weekly recurring slots (UTC) — same module layout
migrations/   golang-migrate SQL files
```

Architecture rules: business logic lives in the service layer; gin handlers only
bind → call the service → render JSON; `*gin.Context` never crosses the handler
boundary. Each domain module owns its types, service, repository port, and
handlers, and mounts its own routes.

## Run it

Prerequisites: Go 1.25+, Docker, and `sqlc` on PATH
(`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`) if you need to regenerate
queries.

```bash
cp .env.example .env
make db-up          # Postgres + Redis via docker compose
make migrate-up
make seed
make run            # API on :8080
make worker         # in another terminal
```

`make db-reset` wipes the volumes and rebuilds a clean, seeded database.

## Endpoints

See `../openapi.yaml`. Currently implemented:

| Method | Path | Notes |
| ------ | ---- | ----- |
| GET | `/healthz` | status + per-dependency checks (200 / 503) |
| GET | `/v1/teachers` | `?language&kind&max_price_minor&q&sort&page&page_size` |
| GET | `/v1/teachers/{slug}` | full profile, 404 if missing |
| GET | `/v1/teachers/{slug}/availability` | weekly recurring slots (UTC), 404 if missing |
| PUT | `/v1/teachers/{slug}/availability` | replace the full weekly set (teacher-owned) |

```bash
curl 'localhost:8080/v1/teachers?language=uz&sort=price_asc'
curl localhost:8080/v1/teachers/nodira-karimova
curl localhost:8080/v1/teachers/nodira-karimova/availability
curl -X PUT localhost:8080/v1/teachers/nodira-karimova/availability \
  -H 'Content-Type: application/json' \
  -H 'X-Teacher-Slug: nodira-karimova' \
  -d '{"slots":[{"weekday":1,"start_minute":540,"end_minute":720}]}'
```

Availability slots are stored in UTC as minutes from 00:00 (`start_minute` /
`end_minute`, aligned to a 15-minute grid); `weekday` is 0 (Sunday) – 6
(Saturday). The teacher's IANA `timezone` (on the profile, echoed in the
availability payload) is the source of truth for converting them to local time
when booking lands. The `PUT` route has no real auth yet — see below.

## Tests

```bash
make test    # go test ./...
make vet
```

`internal/teachers` and `internal/availability` have service tests (fake
repository) and handler tests (httptest). No DB is required for the test suite.

## Not done yet

- **Auth** — `config` has `AUTH_ISSUER` / `AUTH_AUDIENCE` placeholders but no JWT
  middleware. When Clerk/Auth0 is chosen, add verification in `internal/httpapi`
  and a `RequireAuth()` middleware for the mutating routes. Until then
  `PUT /v1/teachers/{slug}/availability` is gated by a stand-in `X-Teacher-Slug`
  header check in the handler (marked with a `TODO(auth)`).
- Booking and payments modules.
- The worker only has a stub `lesson:reminder` handler to show the pattern.
- `cmd/migrate` pulls in golang-migrate's transitive test deps (dktest/docker)
  as indirect modules — a known cost of using it as a library.
