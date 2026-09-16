# FindTutor

An italki-style, two-sided online language-tutoring marketplace, scoped to
**Uzbekistan only**. Students browse teacher profiles, book paid 1-on-1 video
lessons against a teacher's weekly availability, pay for them, attend, and leave
reviews; teachers manage their profile, hours, lessons, and payouts. Prices are
shown in **UZS** (`120,000 so'm`) and the whole product — UI, API error
messages, transactional email — speaks **English, Russian and Uzbek**.
Most teachers are local (`Asia/Tashkent`), so timezone conversion matters mainly
for the few based abroad.

This is a **portfolio project** — it is not deployed and takes no real money. The
payment provider is a fake in-process gateway (see *Status* below).

## Status

**MVP complete.** All six core flows are built, tested, and merged:

- **Auth** — registration, login, session refresh, logout.
- **Teacher profiles** — public catalog with search/filter, per-teacher profile
  pages, self-service profile editing.
- **Weekly availability** — teachers set recurring weekly hours; the API
  projects them into concrete bookable slots.
- **Booking + timezones** — students book a slot; times are shown in both the
  viewer's and the teacher's zone.
- **Payments** — pay for a booking, provider webhook confirms it, refunds on
  cancellation.
- **Lessons + reminders** — meeting links, completion, no-show handling, and
  email reminders sent by a background worker.
- **Reviews** — one review per completed booking, aggregated into a teacher
  rating.

**RBAC admin panel complete** (`/admin`, permission-gated end to end):
dashboard metrics, user directory, roles & permissions management, teacher
moderation (approve / reject / suspend / verify), bookings admin with
force-cancel, lesson-dispute queue, teacher payouts (holding window + payout
runs), and review moderation.

**Learning resources and video courses complete**: a reusable resource library
(materials, articles, auto-graded quizzes, listening/reading tasks,
teacher-graded writing) that attaches to lessons as homework with a grading
inbox, and self-paced video courses (authoring with a curriculum builder, a
public catalog, purchase, a player with progress, revenue-share payouts, admin
moderation).

**Hardening and i18n complete**: rate limiting on the auth surface and uploads
(Redis, shared across instances), trusted-proxy handling, security headers,
request-body caps, upload content sniffing; UI + API messages + email in
en / ru / uz with tests that fail on a missing translation; Prometheus
`/metrics`.

**Payment-webhook signature hardening** is in place (pluggable verifier,
HMAC-SHA256 over `timestamp.body`, replay-safe). A **real Payme / Click / Uzum
adapter is not built** — Stripe does not operate in Uzbekistan and a live
integration needs a merchant account and sandbox. The port and verifier are
ready for one to drop in. Likewise real payout disbursement, cloud object
storage (uploads are on local disk behind a `Blob` port), and CI/Docker images
are parked until the project is meant to ship.

## Architecture

Monorepo with two independently deployable apps and a hand-written API contract
between them.

### `backend/` — Go 1.25 + gin JSON API

- Two binaries: `cmd/api` (the HTTP API) and `cmd/worker` (asynq background jobs
  — lesson reminders and emails). `cmd/migrate` and `cmd/seed` are dev tools.
- **Data:** `sqlc` + `pgx/v5` (pgxpool) over Postgres. Hand-written SQL in
  `internal/db/queries/` is the input; `internal/db/sqlc/` is generated.
- **Migrations:** `golang-migrate` SQL files in `migrations/`, run as a library
  through `cmd/migrate`. Every migration is reversible.
- **Background jobs:** `asynq` on Redis. A nil queue is a guarded no-op, so the
  API and the test suite run without Redis.
- **Layout:** module-per-domain under `internal/` (`auth`, `teachers`,
  `availability`, `bookings`, `payments`, `lessons`, `reviews`, `disputes`,
  `payouts`, `rbac`, `admin`, `files`, `resources`, `courses`, `email`), plus
  cross-cutting `i18n`, `ratelimit`, `web`, `httpapi`. Each module owns its
  `dto.go` / `service.go` / `repository_postgres.go` / `handler.go` and mounts
  its own routes. Business logic lives in the service layer; handlers only bind →
  call the service → render JSON, and `*gin.Context` never crosses the handler
  boundary.
- **Cross-module dependencies** go through ports defined in the *consuming*
  module, with adapters wired in `cmd/api`. `bookings` never imports `payments`,
  `lessons`, or `reviews`.

### `frontend/` — Next.js 16 (App Router) + React 19 + TypeScript

- Tailwind v4 and owned shadcn/ui primitives (`src/components/ui`).
- **Feature modules mirror the backend** (`src/features/<module>/`), each with an
  `api.ts` data-access layer that maps snake_case wire types to camelCase
  view-models — components only ever see view-models.
- **Two HTTP clients, picked by execution context:** `@/lib/api/client` for
  Server Components (no credentials, catalog reads) and
  `@/features/auth/browser-client` for the browser (attaches the access token,
  sends cookies, does exactly one transparent refresh-and-retry on a 401).
- **Auth:** the access token lives in memory only (no `localStorage`);
  `AuthProvider` bootstraps the session with one silent refresh on mount.
  Client-only route guards (`RequireUser`, `RequireAdmin`) gate rendering; the
  backend is the authority on every `/v1/admin/*` call.

### `openapi.yaml` (repo root) — the wire contract

Hand-written and the **source of truth**. The frontend regenerates its types
from it (`cd frontend && npm run gen:api` → `src/lib/api/schema.ts` via
`openapi-typescript`); the backend's DTOs are hand-written to match.

### Conventions

- **Money is integer minor units end to end** (`amount_minor`,
  `price_per_hour_minor`), serialized as `{ amount_minor, currency }`.
- **Availability slots** are stored as integer minutes from `00:00` UTC on a
  15-minute grid, with `weekday` 0=Sunday. Slot→datetime projection is pure UTC
  arithmetic; the teacher's IANA timezone is display-only.
- **Double-booking** is prevented at the DB layer with `btree_gist` `EXCLUDE`
  constraints; a lost race surfaces as SQLSTATE `23P01` → HTTP 409. Webhook and
  review idempotency use insert-first, treat `23505` as "already processed".
- **Auth:** argon2id password hashes, short-lived (~15m) HS256 access JWTs, and
  opaque refresh tokens stored SHA-256-only in `sessions` with rotation and
  reuse detection (replaying a revoked token revokes the whole chain). The
  refresh token rides in the `ftr_session` httpOnly cookie.

## Getting started

You need **Docker Desktop** (for Postgres + Redis), Go 1.25, and Node.js.

```bash
# one-time: backend env file
cd backend && cp .env.example .env
```

Then run three terminals:

```bash
# terminal 1 — database, migrations, seed data, HTTP API on :8080
cd backend && make db-up && make migrate-up && make seed && make run

# terminal 2 — background worker (lesson reminders + emails)
cd backend && make worker

# terminal 3 — frontend on http://localhost:3000
cd frontend && npm install && npm run dev
```

`AUTH_JWT_SECRET` falls back to an insecure dev value when unset (it is required
in production). The worker and Resend/email are optional for basic click-through;
with `RESEND_API_KEY` empty the backend just logs the emails it would send.

### Demo logins (seed data)

| Who | Email | Password |
| --- | --- | --- |
| Admin panel | `admin@findtutor.local` | `admin` |
| Any seed teacher | `<firstname>@example.com`, e.g. `nodira@example.com` | `password` |

A seed teacher can also act as a student by booking a *different* teacher. Fake
payment method tokens: `pm_ok`, `pm_decline`, `pm_capture_fail`.

## Project layout

```
backend/
  cmd/                 api, worker, migrate, seed binaries
  internal/<module>/   one folder per domain (dto/service/repository/handler)
  internal/db/         queries/ (hand-written SQL) + sqlc/ (generated)
  internal/i18n/       message catalogs (ru/uz keyed by the English source) + guards
  internal/dbtest/     integration tests against a real Postgres (build tag `integration`)
  internal/web/        shared HTTP primitives (error envelope, helpers)
  internal/httpapi/    router assembly, middleware, health check, /metrics
  migrations/          golang-migrate SQL files (up + down)
frontend/
  messages/            UI catalogs en/ru/uz (next-intl, typed keys)
  src/app/             Next.js App Router routes (incl. /admin/*)
  src/features/<mod>/  api.ts data layer + module components (mirrors backend)
  src/components/ui/   owned shadcn/ui primitives
  src/i18n/            locale config, request config, switcher action
  src/lib/             api client + schema, format.ts (money), country, i18n helpers
  src/types/           hand-written camelCase view-models
openapi.yaml           hand-written wire contract (source of truth)
```

## Common commands

### Backend (run from `backend/`)

| Command | What it does |
| --- | --- |
| `make db-up` | Start Postgres + Redis via docker compose |
| `make migrate-up` / `make migrate-down` | Apply / roll back one migration |
| `make seed` | Load the demo teacher catalog |
| `make db-reset` | Drop volumes, recreate, migrate, seed — clean slate |
| `make run` | Run the HTTP API on `:8080` |
| `make worker` | Run the background worker |
| `make test` | `go test ./...` (unit suite, no database) |
| `make test-integration` | DB-enforced invariants against a real Postgres (Docker or `TEST_DATABASE_URL`) |
| `make vet` | `go vet ./...` |
| `make sqlc` | Regenerate `internal/db/sqlc` from the query files |

`sqlc` is a standalone binary
(`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`), not a `go tool`
dependency.

### Frontend (run from `frontend/`)

| Command | What it does |
| --- | --- |
| `npm run dev` | Dev server on `http://localhost:3000` (needs the backend on `:8080`) |
| `npm run build` | Production build + full typecheck |
| `npm run lint` | ESLint |
| `npm test` | Vitest — pure logic + message-catalogue checks |
| `npm run gen:api` | Regenerate `src/lib/api/schema.ts` from `../openapi.yaml` |

## Testing

- **Backend:** `make test` needs **no database** — fakes and `httptest`
  throughout. `make test-integration` (build tag `integration`) runs the things
  only Postgres enforces — `EXCLUDE` double-booking, webhook idempotency, the
  jsonb round-trip, ledger `CHECK`s, and every migration's rollback — against a
  throwaway container. `internal/i18n`'s tests fail on any user-facing message
  without a ru/uz translation or with scrambled format verbs.
- **Frontend:** `npm test` (Vitest) covers the pure logic — timezone math,
  formatting, locale negotiation, the auth fetch wrapper's refresh-and-retry —
  and the message catalogues (key parity, ICU syntax, argument/tag parity).
  UI is verified by `npm run build` + `npm run lint` + clicking through
  against the live backend.

## Branches

- **`develop`** — the working branch; all completed work lands here.
- **`main`** — tracks releases.
