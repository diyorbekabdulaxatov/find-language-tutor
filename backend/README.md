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
| POST | `/v1/auth/register` | create account + sign in; 409 on duplicate email |
| POST | `/v1/auth/login` | sign in; 401 on bad credentials |
| POST | `/v1/auth/refresh` | rotate the refresh cookie, new access token |
| POST | `/v1/auth/logout` | revoke session, clear cookie; 204 |
| GET | `/v1/auth/me` | the signed-in user; needs `Authorization: Bearer` |
| PATCH | `/v1/auth/me` | edit own account (`display_name` only; email is read-only) |
| GET | `/v1/teachers` | `?language&kind&max_price_minor&q&sort&page&page_size` |
| POST | `/v1/teachers` | claim/create the caller's profile; Bearer token; 409 if they already own one |
| GET | `/v1/teachers/me` | the caller's own profile; Bearer token; 404 if not created yet |
| GET | `/v1/teachers/{slug}` | full profile, 404 if missing |
| PATCH | `/v1/teachers/{slug}` | edit own profile (partial); Bearer token, must own it; 403/404 otherwise |
| GET | `/v1/teachers/{slug}/availability` | weekly recurring slots (UTC), 404 if missing |
| PUT | `/v1/teachers/{slug}/availability` | replace the full weekly set; Bearer token, must own the profile |

```bash
curl 'localhost:8080/v1/teachers?language=uz&sort=price_asc'
curl localhost:8080/v1/teachers/nodira-karimova/availability

# Auth: seeded teachers all have an account <firstname>@example.com / "password".
ACCESS=$(curl -s -X POST localhost:8080/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"nodira@example.com","password":"password"}' | jq -r .access_token)

curl localhost:8080/v1/auth/me -H "Authorization: Bearer $ACCESS"
curl -X PATCH localhost:8080/v1/auth/me \
  -H "Authorization: Bearer $ACCESS" -H 'Content-Type: application/json' \
  -d '{"display_name":"Nodira K."}'

# Own teacher profile (404 until created), then create / edit it.
curl localhost:8080/v1/teachers/me -H "Authorization: Bearer $ACCESS"
curl -X POST localhost:8080/v1/teachers \
  -H "Authorization: Bearer $ACCESS" -H 'Content-Type: application/json' \
  -d '{"display_name":"New Teacher","headline":"Conversational English",
       "kind":"community","country_code":"UZ","country_name":"Uzbekistan",
       "city":"Tashkent","timezone":"Asia/Tashkent","price_per_hour_minor":6000000,
       "currency":"UZS",
       "languages":[{"role":"teaches","code":"en","name":"English","level":"c1"}]}'
curl -X PATCH localhost:8080/v1/teachers/new-teacher \
  -H "Authorization: Bearer $ACCESS" -H 'Content-Type: application/json' \
  -d '{"headline":"IELTS & Business English","focus":["IELTS","Business"]}'

curl -X PUT localhost:8080/v1/teachers/nodira-karimova/availability \
  -H "Authorization: Bearer $ACCESS" -H 'Content-Type: application/json' \
  -d '{"slots":[{"weekday":1,"start_minute":540,"end_minute":720}]}'
```

Auth model: access tokens are short-lived (15 min) HS256 JWTs held in memory by
the client; refresh tokens are opaque, 30-day, and delivered as the HttpOnly
`ftr_session` cookie (`Path=/v1/auth`, `SameSite=Lax`, `Secure` outside dev).
`/refresh` rotates the cookie and revokes the previous token; presenting an
already-revoked token revokes every session for that user. Passwords are
argon2id (64 MiB / t=1 / p=4). Browser clients must use `credentials: 'include'`
on the auth calls. Config: `AUTH_JWT_SECRET` (required in production),
`AUTH_ACCESS_TTL`, `AUTH_REFRESH_TTL`, `AUTH_COOKIE_DOMAIN`, `AUTH_COOKIE_SECURE`.

**Demo accounts are seeded with the password `password` — demo only, never a
real-account pattern.**

Availability slots are stored in UTC as minutes from 00:00 (`start_minute` /
`end_minute`, aligned to a 15-minute grid); `weekday` is 0 (Sunday) – 6
(Saturday). The teacher's IANA `timezone` (on the profile, echoed in the
availability payload) is the source of truth for converting them to local time
when booking lands. `PUT` requires a Bearer access token whose user owns the
teacher row (`teachers.user_id`): 401 without a valid token, 403 if not the owner.

## Tests

```bash
make test    # go test ./...
make vet
```

`internal/auth`, `internal/teachers` and `internal/availability` have service
tests (fake repository) and handler tests (httptest). No DB is required for the
test suite.

## Not done yet

- Email verification, password reset, and rate-limiting on the auth endpoints.
- Changing a profile's `slug` (immutable for now), and clearing a trial price
  via `PATCH` (an omitted `trial_price_minor` is left unchanged; there is no way
  yet to express "remove the trial").
- Booking and payments modules.
- The worker only has a stub `lesson:reminder` handler to show the pattern.
- `cmd/migrate` pulls in golang-migrate's transitive test deps (dktest/docker)
  as indirect modules — a known cost of using it as a library.
