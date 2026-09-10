# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`FindTutor` — an italki-style two-sided online language-tutoring marketplace, scoped to **Uzbekistan only**. Students book paid 1-on-1 video lessons with teachers. Prices shown in **UZS** (`120,000 so'm`), UI in **English** (no i18n yet). Most teachers are local (`Asia/Tashkent`), so timezone conversion matters mainly for the few abroad.

Monorepo, two independently deployable apps plus a hand-written contract:

- `backend/` — Go 1.25 + gin JSON API. Two binaries: `cmd/api` (HTTP) and `cmd/worker` (asynq jobs).
- `frontend/` — Next.js 16 (App Router) + React 19 + TypeScript + Tailwind v4.
- `openapi.yaml` (repo root) — **hand-written, the source of truth** for the wire contract.

All active work is on the **`develop`** branch. `main` is still the initial commit. The MVP is built phase by phase; backend sub-tasks run in agent worktrees under `.claude/worktrees/` (gitignored). **Never create files or folders outside this project directory** (firm user rule).

## The openapi.yaml contract

`openapi.yaml` is edited by hand. It drives both sides but generates only one of them:

- **Backend** DTOs are hand-written to match it (snake_case, money as `{amount_minor, currency}`). Changing the contract means editing Go DTOs by hand too.
- **Frontend** runs `cd frontend && npm run gen:api` to regenerate `src/lib/api/schema.ts` via `openapi-typescript`. Do this after any `openapi.yaml` change.

Gotcha: `required`-only object schemas make `openapi-typescript` emit `Record<string, never>` and break the type — write request bodies as a direct `$ref` to a writable schema, not an `allOf` wrapper.

## Backend

### Commands (run from `backend/`)

```bash
make db-up          # Postgres + Redis via docker compose (needs Docker Desktop)
make migrate-up     # apply migrations (go run ./cmd/migrate up)
make migrate-down   # roll back one
make seed           # load the demo teacher catalog
make db-reset       # wipe volumes, migrate, seed — clean slate
make run            # HTTP API on :8080
make worker         # background worker (reminders/emails) — separate terminal
make test           # go test ./...
make vet
make sqlc           # regenerate internal/db/sqlc from internal/db/queries (needs sqlc on PATH)

go test ./internal/bookings/ -run TestSlotGeneration -v   # single package / test
```

`sqlc` is a standalone binary (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`), deliberately **not** a `go tool` dependency. The test suite needs **no database** (fakes + httptest throughout). Copy `.env.example` to `.env` for local dev; `AUTH_JWT_SECRET` falls back to an insecure dev value when unset (required in production).

### Architecture

**Module pattern.** Each domain lives in `internal/<module>/` and owns its `dto.go` / `service.go` / `repository_postgres.go` / `handler.go` and a `RegisterRoutes` function that mounts its own routes. Modules: `auth`, `teachers`, `availability`, `bookings`, `payments`, `reviews`, `email`, `lessons`. Shared HTTP primitives (error envelope, response helpers) live in `internal/web` so `httpapi` and domain modules don't cycle. `internal/httpapi` assembles the router, middleware, and health check.

**Layering rule.** Business logic is in the service layer. Handlers only bind → call the service → render JSON. `*gin.Context` **never crosses the handler boundary** — services take `context.Context` and plain args.

**Cross-module dependencies go through ports defined in the *consuming* module**, with adapters wired in `cmd/api`. `bookings` never imports `payments`, `lessons`, or `reviews`; instead it declares `bookings.PaymentGateway`, `bookings.ReminderScheduler`, `bookings.Notifier`, `bookings.ReviewReader`, and `cmd/api` injects the implementations (`payments.NewGateway(...)`, `lessons.NewScheduler(...)`, `reviews.NewBookingGateway(...)`) via `bookingService.Set*(...)`. A nil port is a guarded no-op, so `cmd/api` without Redis and the unit tests still work. The reverse direction (a payment authorization moving a booking to `confirmed`) happens inside the payments webhook transaction as a guarded `UPDATE` on the bookings row.

**Data.** `sqlc` + `pgx/v5` (pgxpool). Hand-written SQL in `internal/db/queries/` is the input; `internal/db/sqlc/` is generated — never edit it. Migrations are `golang-migrate` SQL files in `migrations/`, run as a *library* through `cmd/migrate` (not the CLI). Every migration must be reversible (down-tested).

**Conventions.**
- Money is **integer minor units** end to end (`amount_minor`, `price_per_hour_minor`).
- Availability slots are stored as **integer minutes from 00:00 UTC** (`start_minute` / `end_minute`, 15-min grid), `weekday` 0=Sunday..6=Saturday. Slot→datetime projection is pure UTC arithmetic; the teacher's IANA timezone is display-only and never enters the math.
- Double-booking is prevented at the **DB layer** with `btree_gist` `EXCLUDE` constraints; a lost race surfaces as SQLSTATE `23P01` → a 409. Same idea for webhook/review idempotency: `INSERT` first, treat `23505` as "already processed" — never check-then-insert.
- Auth: argon2id passwords, short-lived HS256 access JWTs (~15m), opaque refresh tokens stored SHA-256-only in `sessions` with rotation + reuse-detection (replaying a revoked token revokes the whole chain). Refresh token rides in the `ftr_session` httpOnly cookie (`Path=/v1/auth`, `SameSite=Lax`). `RequireAuth` / `OptionalAuth` middleware.
- `cmd/seed` mirrors the frontend's demo data; FKs have no `ON DELETE CASCADE`, so seed clears tables in dependency order. Keep the seed coherent with every new migration. Demo teachers all have an account `<firstname>@example.com` / `password` (demo only).

## Frontend

### Commands (run from `frontend/`)

```bash
npm install
npm run dev        # http://localhost:3000 (needs the backend on :8080)
npm run build      # production build + typecheck
npm run lint       # eslint (no separate tsc step; build does the full typecheck)
npm run gen:api    # regenerate src/lib/api/schema.ts from ../openapi.yaml
```

There is no frontend unit-test setup — verification is `npm run build` + `npm run lint` + clicking through against the live backend.

### Architecture

**Feature modules mirror the backend.** `src/features/<module>/` (`auth`, `teachers`, `dashboard`, `availability`, `bookings`, `reviews`, `admin`) holds an `api.ts` data-access layer plus module-specific components. `src/components/ui` is owned shadcn/ui primitives (radix-nova style; `cn` is imported from the `cn` npm package, not a local util). `src/lib` has `format.ts` (money), `country.ts`, `i18n.ts`. `src/types/` holds hand-written **camelCase view-models**.

**Two HTTP clients — pick by execution context:**
- `@/lib/api/client` (`api`) — for **Server Components**. Uses `API_URL` (no `NEXT_PUBLIC_` prefix), no credentials. Most teacher/catalog reads.
- `@/features/auth/browser-client` (`browserApi`, `authedFetch`) — for the **browser**. Uses `NEXT_PUBLIC_API_URL`, attaches the in-memory access token as a Bearer header, sends `credentials: "include"`, and does exactly **one** transparent refresh-and-retry on a 401 (concurrent 401s share a single refresh call).

**`api.ts` files map snake_case wire types → camelCase view-models** (`toSummary`, `toProfile`, `toMoney`, …). Components only ever see view-models. When an endpoint isn't in `openapi.yaml` yet (e.g. in-progress `/v1/admin/*`), `api.ts` uses `authedFetch` with hand-written wire types and a `// swap to browserApi once the schema regenerates` note.

**Auth flow.** Access token lives **in memory only** (`auth-store.ts`, an external store — no `localStorage`). `AuthProvider` (in the root layout, under `ThemeProvider`) bootstraps the session with one silent `refreshSession()` on mount. `useAuth()` → `{user, status, login, register, logout, setUser}`. Client-only route guards: `RequireUser` (→ `/login?next=`), `RequireAdmin`. `?next=` is always validated as a local path.

**Timezone.** `src/features/availability/timezone.ts` converts the teacher's local weekly hours ↔ the backend's UTC minutes-from-midnight, splitting a slot that straddles UTC midnight into two and merging them back. Booking UI (`src/features/bookings/datetime.ts`) is `Intl`-only, showing times in both the viewer's and the teacher's zone.

**Admin RBAC.** `src/features/admin/permissions.ts` mirrors the backend's permission keys; the backend is the authority (every `/v1/admin/*` call is checked server-side), the frontend gates only render UI (`use-can.ts`, `PermissionGate`).

### Next.js 16 specifics

- This is **not** the Next.js in your training data. Bundled docs live in `frontend/node_modules/next/dist/docs/` — read the relevant one before assuming an API. `frontend/AGENTS.md` (re-injected by `next dev`; commit it with your work to keep the tree clean) says the same.
- `params` / `searchParams` are **Promises** — `await` them. Route prop types are the global `PageProps<'/route'>` / `LayoutProps<'/route'>`.
- Lint rules enforced here that commonly bite: `react-hooks/set-state-in-effect` (don't `setState` synchronously in an effect body — wrap in an inner `async function load(){…}`) and `react-hooks/purity` (no `Date.now()` / `new Date()` in render — use `useState(() => Date.now())`).

### Design direction

"Bold modern marketplace", green-forward. Tokens (light + dark) in `src/app/globals.css`: emerald `--primary`, coral trial CTAs, mint for online/verified, amber stars, white cards on a faint green ground. Fonts: **Bricolage Grotesque** (display, `--font-display`) + **Plus Jakarta Sans** (body, `--font-sans`), self-hosted via `next/font`. Dark theme via `next-themes` (`attribute="class"`). The `frontend-design` skill in `.claude/skills/` has the fuller rationale. (Note: older "warm paper / marigold / Fraunces" notes in `frontend/README.md` are stale — match `globals.css`.)

## Local dev, end to end

```bash
# terminal 1
cd backend && make db-up && make migrate-up && make seed && make run
# terminal 2 (reminders + emails)
cd backend && make worker
# terminal 3
cd frontend && npm run dev
```

Log in with any seed teacher (`nodira@example.com` / `password`). A seed teacher can act as a student by booking a *different* teacher. Fake payment method tokens: `pm_ok`, `pm_decline`, `pm_capture_fail`. The backend's `ALLOWED_ORIGINS` already lists `http://localhost:3000`.

## Workflow expectation

The MVP is built phase by phase. After each phase: review the diff (architecture + security + correctness), run the tests, click/curl through it, then merge to `develop` and report before starting the next. All 6 MVP phases (auth, availability, booking, payments, lessons, reviews) are done; current work is the admin panel and cross-cutting polish.
