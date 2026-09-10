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
  bookings/   concrete scheduled lessons — slots, booking lifecycle, meeting links, no-show
  payments/   payment intents + fake provider + payout ledger
  reviews/    phase-6: lesson reviews + incremental teacher-rating aggregate (bookings.ReviewReader port)
  disputes/   phase-D: lesson disputes — participant routes on /v1/bookings, the
              operator queue on /v1/admin (bookings.DisputeReader port)
  email/      transactional email (Resend / logging backend) + booking templates
  lessons/    phase-5 wiring: asynq reminder scheduler + email notifier (bookings ports)
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
| GET | `/v1/auth/me` | the signed-in user + `permissions` (RBAC keys, resolved per request, `[]` for a normal user); needs `Authorization: Bearer` |
| PATCH | `/v1/auth/me` | edit own account (`display_name` only; email is read-only) |
| GET | `/v1/teachers` | `?language&kind&max_price_minor&q&sort&page&page_size` — **only `approved` teachers** |
| POST | `/v1/teachers` | claim/create the caller's profile; Bearer token; 409 if they already own one. **New profiles are `status = pending`** — fillable + can set availability, but not public until an admin approves |
| GET | `/v1/teachers/me` | the caller's own profile **regardless of status**; includes `status` / `verified` / `moderation_note`; Bearer token; 404 if not created yet |
| GET | `/v1/teachers/{slug}` | full profile; **404 unless `approved`**; carries `verified` (badge) |
| PATCH | `/v1/teachers/{slug}` | edit own profile (partial); Bearer token, must own it; 403/404 otherwise. Now also accepts `meeting_url` (default video room; http(s) or empty) |
| GET | `/v1/teachers/{slug}/availability` | weekly recurring slots (UTC), 404 if missing |
| PUT | `/v1/teachers/{slug}/availability` | replace the full weekly set; Bearer token, must own the profile |
| GET | `/v1/teachers/{slug}/slots` | `?from&to&duration` — concrete bookable start times (UTC); public; **404 for a non-`approved` slug**; 400 if the window > 21 days |
| GET | `/v1/teachers/{slug}/reviews` | `?page&page_size` (default 1 / 10, cap 50) — the teacher's reviews, newest first; public; **404 unless `approved`** |
| POST | `/v1/bookings` | book a lesson; Bearer token; **404 for a non-`approved` teacher**; 201 `pending_payment` + opens a `requires_payment` intent; 409 `slot_unavailable` / `slot_taken` |
| GET | `/v1/bookings` | `?role=student\|teacher&status=` — the caller's bookings, newest first; Bearer token |
| GET | `/v1/bookings/{id}` | full booking (with embedded `payment`); Bearer token; 404 if missing, 403 if not a participant |
| POST | `/v1/bookings/{id}/pay` | `{method_token}`; student only; authorize → `pending_payment → confirmed`; 402 `payment_failed`, 409 `already_paid` |
| POST | `/v1/bookings/{id}/complete` | teacher-owner only; `confirmed → completed` + capture + payout-ledger row; 409 `too_early`, 502 `capture_failed` |
| POST | `/v1/bookings/{id}/cancel` | `{reason?}`; `pending_payment\|confirmed → cancelled`; participant only; refunds/voids the intent; cancels reminders + emails the other party; 409 if already done |
| PUT | `/v1/bookings/{id}/meeting-link` | `{url}`; teacher-owner only; sets the per-booking link override (empty clears it); 403/404 |
| POST | `/v1/bookings/{id}/no-show` | `{party}`; teacher-owner only; from `confirmed` once started (409 `too_early`); `student` → `completed` + capture, `teacher` → `cancelled` + refund |
| POST | `/v1/bookings/{id}/review` | `{rating: 1-5, comment?}`; student only; booking must be `completed` (409 `booking_not_completed`); one per booking (409 `already_reviewed`); nudges the teacher `rating` / `review_count` in the same transaction; 201 |
| POST | `/v1/payments/webhook` | provider event; unauthenticated (signed); idempotent by `event_id`; 200 on a well-formed duplicate |
| GET | `/v1/payments/me` | the caller's teacher earnings summary; Bearer token; 404 if they own no profile |
| GET | `/v1/admin/metrics` | dashboard counters; perm `metrics.view` |
| GET | `/v1/admin/users` | `?q&page&page_size` — user directory; perm `users.view` |
| GET | `/v1/admin/users/{id}` | user + roles + teacher profile + 50 newest bookings + payments summary; perm `users.view` |
| POST/DELETE | `/v1/admin/users/{id}/roles[/{role_id}]` | assign / unassign a role (idempotent); perm `users.manage_roles` |
| GET | `/v1/admin/permissions` | the permission catalog; perm `roles.manage` |
| GET/POST | `/v1/admin/roles` | list / create roles; perm `roles.manage`; 400 `unknown_permission`, 409 `role_exists` |
| PATCH/DELETE | `/v1/admin/roles/{id}` | edit (description always; permissions unless `is_system` → 403 `role_locked`) / delete (403 `role_locked`, 409 `role_in_use`); perm `roles.manage` |
| GET | `/v1/admin/teachers` | `?status&q&page&page_size` — moderation queue (all statuses); perm `teachers.view` |
| GET | `/v1/admin/teachers/{slug}` | full profile + `status` / `verified` / `moderation_note` / `owner`; perm `teachers.view` |
| POST | `/v1/admin/teachers/{slug}/approve` | → `approved`, clears note; from pending/rejected/suspended; 409 `invalid_transition`; perm `teachers.moderate` |
| POST | `/v1/admin/teachers/{slug}/reject` | `{note}` (required) → `rejected`; from pending only; perm `teachers.moderate` |
| POST | `/v1/admin/teachers/{slug}/suspend` | `{note}` (required) → `suspended`; from approved only; leaves bookings intact; perm `teachers.moderate` |
| POST | `/v1/admin/teachers/{slug}/verify` | `{verified: bool}`; independent of status; perm `teachers.verify` |
| GET | `/v1/admin/bookings` | `?status&q&page&page_size` — every booking on the platform, newest lesson first; perm `bookings.view` |
| GET | `/v1/admin/bookings/{id}` | full booking + payment + meeting link + cancellation who/why/when + the dispute thread; perm `bookings.view`; 404 `booking_not_found` |
| POST | `/v1/admin/bookings/{id}/force-cancel` | `{reason, refund?}` — operator override of the participant-only cancel; records `cancelled_by = admin`; 409 `invalid_state`; perm `bookings.force_cancel` |
| POST | `/v1/bookings/{id}/disputes` | `{reason}`; participant only; booking must be `confirmed`/`completed` (409 `dispute_not_allowed`); one open dispute per booking (409 `dispute_exists`); 201 |
| GET | `/v1/bookings/{id}/disputes` | the booking's whole dispute thread, newest first; participant only |
| GET | `/v1/admin/disputes` | `?status=open\|resolved\|rejected\|all&page&page_size` (default `open`) — the operator queue with booking + parties; perm `disputes.resolve` |
| POST | `/v1/admin/disputes/{id}/resolve` | `{outcome: resolved\|rejected, resolution, refund?}`; 404 `dispute_not_found`, 409 `already_resolved`; perm `disputes.resolve` |

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

# Concrete bookable slots (public), then book / confirm / cancel as a student.
curl 'localhost:8080/v1/teachers/nodira-karimova/slots?duration=60'

STU=$(curl -s -X POST localhost:8080/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"sardor@example.com","password":"password"}' | jq -r .access_token)

BID=$(curl -s -X POST localhost:8080/v1/bookings -H "Authorization: Bearer $STU" \
  -H 'Content-Type: application/json' \
  -d '{"teacher_slug":"nodira-karimova","start_at":"2026-09-14T09:00:00Z","duration_minutes":60}' | jq -r .id)

curl "localhost:8080/v1/bookings?role=student" -H "Authorization: Bearer $STU"

# Pay (student). Fake-provider method tokens: pm_ok | pm_decline | pm_capture_fail.
curl -X POST "localhost:8080/v1/bookings/$BID/pay" -H "Authorization: Bearer $STU" \
  -H 'Content-Type: application/json' -d '{"method_token":"pm_ok"}'

# Complete (teacher-owner, only after the lesson's end_at), then check earnings.
curl -X POST "localhost:8080/v1/bookings/$BID/complete" -H "Authorization: Bearer $ACCESS"
curl localhost:8080/v1/payments/me -H "Authorization: Bearer $ACCESS"

curl -X POST "localhost:8080/v1/bookings/$BID/cancel" -H "Authorization: Bearer $STU" \
  -H 'Content-Type: application/json' -d '{"reason":"schedule clash"}'

# Admin: seeded account admin@findtutor.local / "admin" (superadmin role).
ADMIN=$(curl -s -X POST localhost:8080/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@findtutor.local","password":"admin"}' | jq -r .access_token)

curl localhost:8080/v1/admin/metrics -H "Authorization: Bearer $ADMIN"
curl 'localhost:8080/v1/admin/teachers?status=pending' -H "Authorization: Bearer $ADMIN"
curl -X POST localhost:8080/v1/admin/teachers/malika-abdurakhmonova/approve \
  -H "Authorization: Bearer $ADMIN"
curl -X POST localhost:8080/v1/admin/teachers/malika-abdurakhmonova/verify \
  -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' -d '{"verified":true}'

# Bookings admin + disputes (phase D). The seed leaves one open dispute.
curl 'localhost:8080/v1/admin/bookings?status=confirmed' -H "Authorization: Bearer $ADMIN"
DID=$(curl -s localhost:8080/v1/admin/disputes -H "Authorization: Bearer $ADMIN" | jq -r '.disputes[0].id')
BOOKED=$(curl -s localhost:8080/v1/admin/disputes -H "Authorization: Bearer $ADMIN" | jq -r '.disputes[0].booking.id')
curl "localhost:8080/v1/admin/bookings/$BOOKED" -H "Authorization: Bearer $ADMIN"
curl -X POST "localhost:8080/v1/admin/disputes/$DID/resolve" \
  -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
  -d '{"outcome":"resolved","resolution":"Refunded the lesson.","refund":true}'

# A participant raises a dispute; an operator can force-cancel a live booking.
curl -X POST "localhost:8080/v1/bookings/$BID/disputes" -H "Authorization: Bearer $STU" \
  -H 'Content-Type: application/json' -d '{"reason":"The teacher never joined."}'
curl -X POST "localhost:8080/v1/admin/bookings/$BID/force-cancel" \
  -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' \
  -d '{"reason":"Duplicate booking","refund":true}'

# RBAC: create a scoped role, assign it, and the holder logs in to see permissions.
curl localhost:8080/v1/admin/roles -H "Authorization: Bearer $ADMIN"
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

Bookings (`internal/bookings`) turn that recurring availability into concrete
lessons. `GET /v1/teachers/{slug}/slots` projects each weekly span (UTC
minutes-from-midnight on a UTC weekday) onto real UTC datetimes across the
requested window — the projection is pure UTC arithmetic, the teacher's IANA
timezone is echoed for display only and never enters the math. Spans the
frontend split at 00:00 UTC (a teacher whose local hours wrap midnight) are
stitched back together before candidate starts are stepped every 30 minutes, so
a lesson may legitimately straddle UTC midnight. A start is offered only if the
whole `[start, start+duration]` fits one availability window and clears every
non-cancelled booking; `duration` is one of 30/60/90/120 (default 60) and the
window is capped at 21 days. Slot price is
`round(price_per_hour_minor * duration / 60)` in integer minor units (half-up).

`POST /v1/bookings` re-runs that exact check server-side (never trusting the
client's price or alignment), then inserts `status='pending_payment'`.
Double-booking is prevented at the DB layer by two `EXCLUDE USING gist`
constraints (one on `teacher_id`, one on `student_id`, both over
`tstzrange(start_at, end_at)` where `status <> 'cancelled'`): a slot the service
can already see as taken returns 409 `slot_unavailable`, and a lost race on the
constraint returns 409 `slot_taken`. `cancel` is participant-only
(student or teacher-owner); cancellation is currently allowed at any time and
only records who/when (`// TODO(phase-5): cancellation window / penalties`).

Payments (`internal/payments`) sit behind a provider-agnostic port
(`payments.Provider` — `Authorize` / `Capture` / `Refund`). The MVP ships a
deterministic in-process **fake** (`fake_provider.go`); Stripe does not operate
in Uzbekistan, and a real Payme / Click / Uzum adapter drops in behind the same
port later. The fake's `method_token` drives the outcome: `pm_ok` authorizes,
`pm_decline` is refused (402 `payment_failed`), `pm_capture_fail` authorizes but
fails the first `Capture`.

Module boundary: `internal/bookings` defines the port it needs
(`bookings.PaymentGateway`); `internal/payments` provides the adapter
(`payments.NewGateway`), wired in `cmd/api` with `bookingService.SetPaymentGateway`.
`bookings` never imports `payments`. The reverse direction — a successful
authorization moving the booking `pending_payment → confirmed` — happens inside
the webhook transaction as a guarded `UPDATE` on the bookings row, so payment
and booking state commit together.

Every provider state change is a **webhook event** (`payment.authorized`,
`payment.captured`, `payment.refunded`, `payment.failed`) with a stable
`event_id`. The in-process fake routes events through the same
`Service.HandleWebhook` a real provider's HTTP `POST /v1/payments/webhook` would
hit. Idempotency is enforced by the `payment_events` **primary key**: the
handler inserts `event_id` first and treats a `23505` unique violation as
"already processed" — never a check-then-insert, so concurrent duplicate
deliveries are race-safe. A well-formed duplicate still returns 200.

The `payout_ledger` is a simplified teacher-earnings model: one row per captured
booking, `held` on capture then `available` immediately (no clearing window for
the MVP — `// TODO(payouts)`), `reversed` on refund. `GET /v1/payments/me`
aggregates it into `{total_earned_minor, held_minor, available_minor, currency,
lessons[]}`; reversed lessons are excluded from the totals. Money is integer
minor units end to end.

Config: `PAYMENTS_PROVIDER` (default `fake`), `PAYMENTS_WEBHOOK_SECRET` (when
set, the webhook verifies an `X-Payment-Signature` HMAC-SHA256 header; empty
disables verification for dev).

### Lessons: meeting links, no-show, reminders, email (phase 5)

**Meeting links.** A teacher has a default `meeting_url` (`PATCH
/v1/teachers/{slug}`), and a booking can carry a `meeting_url_override` (`PUT
/v1/bookings/{id}/meeting-link`, teacher-owner only, empty string clears it).
The *effective* link (override if set, else the teacher default) is exposed on
`Booking.meeting_url` **only when the caller is a participant AND the booking is
`confirmed` or `completed`** — it never leaks to a `pending_payment` booking or
a non-participant. The rule lives in one place, `meetingURLFor` in
`internal/bookings/dto.go`, and every response goes through it.

**No-show** (`POST /v1/bookings/{id}/no-show`, teacher-owner only, MVP). Allowed
from `confirmed` once `start_at` is past (else 409 `too_early`).
`party="student"` reuses the phase-4 complete/capture path (→ `completed`,
`no_show_party=student`, payment captured, payout-ledger row);
`party="teacher"` reuses the cancel/refund path (→ `cancelled`,
`no_show_party=teacher`, student refunded). Both drop any pending reminders.

**Reminders.** `bookings.ReminderScheduler` (a port, like `PaymentGateway`) is
implemented in `internal/lessons` over an `*asynq.Client` + `*asynq.Inspector`.
`Pay` → `Schedule` enqueues **two** `lesson:reminder` tasks with deterministic
ids `reminder:24h:<id>` / `reminder:1h:<id>` at `ProcessAt(start-24h)` /
`ProcessAt(start-1h)` on the `default` queue; a run time already in the past is
skipped. `Cancel` (`cancel` / teacher no-show) deletes both ids via the
inspector (not-found is fine). "Reschedule" = `Cancel` then `Schedule`.

**Worker** (`cmd/worker`). The `lesson:reminder` handler loads the booking,
sends only if it is still `confirmed` (cancelled/completed → log + no retry),
renders the `lesson_reminder` template with the effective meeting link, and
mails the student + teacher; a send error is returned so asynq retries.

**Email** (`internal/email`). `Emailer.Send(ctx, Message)` with two backends:
`resendEmailer` (POSTs `https://api.resend.com/emails`, `Authorization: Bearer
RESEND_API_KEY`, `from` = `EMAIL_FROM`) and `logEmailer` (logs to / subject /
first body line at INFO). `email.New` picks `logEmailer` when `RESEND_API_KEY`
is empty (the dev default). Templates: **booking_confirmed** (both participants,
right after `pay`), **booking_cancelled** (the other party, on cancel / teacher
no-show, with a refund note), **lesson_reminder** (worker; one template, a
`Kind` selects "in 24 hours" / "in 1 hour"). All times render in the **teacher's
timezone** — a student account has no timezone yet, so student mail also uses
the teacher's tz and shows the zone name so it is unambiguous. `internal/lessons`
implements `bookings.Notifier`, maps a `bookings.Booking` to the template data,
and fans each message out to both participants.

Both ports are optional on the booking service: a nil `ReminderScheduler` /
`Notifier` makes every call a guarded no-op, so `cmd/api` without Redis and the
unit tests keep working. Config: `RESEND_API_KEY` (empty → `logEmailer`),
`EMAIL_FROM` (default `findtutor <noreply@findtutor.local>`).

### Reviews (phase 6)

`internal/reviews`. `POST /v1/bookings/{id}/review` is student-only, requires the
booking to be `completed`, and allows one review per booking. Idempotency is the
DB's job: `reviews_booking_uniq` is a partial unique index on `booking_id` (the
seeded booking-less sample rows are exempt), the repository inserts and maps
SQLSTATE `23505` to `already_reviewed` — race-safe, never a check-then-insert,
same pattern as `payment_events`.

The insert and the teacher-aggregate bump run in **one transaction**:
`review_count = review_count + 1`, `rating = round((rating*review_count +
new_rating) / (review_count + 1), 1)` clamped to `[0, 5]` (all references see
the pre-`UPDATE` row, so `review_count` is the old count in both terms). The
hand-set seed `rating` / `review_count` are the historical baseline a new review
nudges; the seeded sample reviews (`booking_id NULL`, ~3–4 per teacher) do
**not** touch the aggregate — they display as "showing 4 of 214".

The reviews module exposes `bookings.ReviewReader` (mirrors `PaymentGateway`)
back to bookings so a `BookingDTO` carries `can_review` (student + `completed` +
not yet reviewed) and `review` (`{rating, comment, created_at} | null`, visible
to both participants) without a second call. bookings never imports reviews;
`cmd/api` injects the adapter with `bookingService.SetReviewReader(...)`. A nil
reader is a guarded no-op.

### Admin & RBAC (phase A/B)

`internal/rbac` + `internal/admin`, mounted under `/v1/admin` behind
`auth.RequireAuth`; each route then adds its own `rbacGuard.Require("<perm>")`.

**RBAC model** (migration `000008_rbac`): `roles`, `role_permissions`,
`user_roles`. A user's effective permissions are the **union across their roles**,
resolved **from the database on every request** — deliberately *not* in the JWT,
so revoking a role takes effect immediately rather than at the next 15-minute
token refresh. The access-token claims are unchanged (`sub` / `email` /
`display_name`). `internal/rbac/permissions.go` is the permission catalog (the
source of truth): `role_permissions` writes are validated against it, and
`GET /v1/admin/permissions` serves it. A+B wire `metrics.view`, `users.view`,
`users.manage_roles`, `teachers.view`, `teachers.moderate`, `teachers.verify`,
`roles.manage`; the rest (`bookings.*`, `disputes.resolve`, `payouts.*`,
`reviews.moderate`) are defined for later phases.

- **`GET /v1/auth/me`** and the login / register / refresh responses carry the
  caller's `permissions` (flat, sorted). rbac implements `auth.PermissionsPort`;
  a nil port yields `[]`.
- **`superadmin`** is a system role (`is_system = true`) that always holds the
  **entire catalog**: the seed grants every `rbac.AllPermissions` entry, and
  `Service.PermissionsFor` short-circuits to `AllPermissions` for any holder of
  the `superadmin` role even if the stored `role_permissions` rows lag a catalog
  change. System roles can't be renamed, re-permissioned, or deleted through the
  API (`403 role_locked`); their description is editable.
- Per-request resolution path: `auth.RequireAuth` stashes the user id →
  `rbacGuard.Require(perm)` calls `Service.PermissionsFor(userID)` (two indexed
  queries: `user_roles ⋈ roles` for the superadmin guard, `user_roles ⋈
  role_permissions` for the union) → `403 forbidden` unless `perm` is in the set.
  The resolved set is stashed so `rbac.Can(c, perm)` works in-handler.

**Teacher moderation** (migration `000009_teacher_moderation`): `teachers`
gains `status` (`pending` | `approved` | `rejected` | `suspended`, **DEFAULT
`approved`** so every existing/seeded row stays live), `verified`, and
`moderation_note`. `POST /v1/teachers` overrides the default to `pending`. Public
reads hide everything non-`approved`:

| read | how |
| --- | --- |
| `GET /v1/teachers` (list + count) | `WHERE t.status = 'approved'` in `ListTeachers` / `CountTeachers` (always, not a filter param) |
| SSG slug list | `WHERE status = 'approved'` in `ListTeacherSlugs` |
| `GET /v1/teachers/{slug}` | repo returns the row; `teachers.Service.GetBySlug` maps non-`approved` → `ErrNotFound` (handler 404). `GetForAdmin` / `GetOwnProfile` skip that check |
| `GET /v1/teachers/{slug}/slots`, `POST /v1/bookings` | `GetBookingTeacherContext` query has `AND status = 'approved'` → `ErrTeacherNotFound` → **404** |
| `GET /v1/teachers/{slug}/reviews` | reviews repo uses `ApprovedTeacherIDBySlug` (`AND status = 'approved'`) → 404 |

Owner reads (`GET /v1/teachers/me`) and `PUT .../availability` are unaffected — a
`pending` teacher fills in everything and sets availability while invisible.

**Admin repo** reads other modules' tables directly
(`internal/db/queries/admin.sql`) — it's an ops tool, not a public API. The one
exception is `GET /v1/admin/teachers/{slug}`, which reuses the teachers read
model via the `admin.TeacherProfiles` port (`*teachers.Service`).

**Seed**: 3 roles (`superadmin` system + `support` + `moderator` examples), one
admin account **`admin@findtutor.local` / `admin`** holding `superadmin` (no
teacher profile), 3 verified seed teachers (Nodira, Elena, Kim), and one
`pending` demo teacher (`malika-abdurakhmonova`, account `malika@example.com` /
`password`) so the moderation queue isn't empty on a fresh DB.

### Bookings admin, force-cancel & disputes (phase D)

Migration `000010_disputes` adds **`bookings.cancelled_by`** (`''` | `student` |
`teacher` | `admin`) and the **`disputes`** table.

**Bookings admin.** `GET /v1/admin/bookings` (`bookings.view`) lists every
booking on the platform — the participant `GET /v1/bookings` only ever shows the
caller's — with `?status` and a `?q` that matches the teacher's display name /
slug and the student's email / display name. Each row carries both parties, the
money, the payment status (`null` when there is no intent yet) and
`has_open_dispute`. `GET /v1/admin/bookings/{id}` adds the payment detail, the
effective meeting link (operators see it at any status, unlike participants),
`no_show_party`, the cancellation who/why/when, and the full dispute thread. All
of it is read straight from the tables via `admin.sql`, per the admin-repo rule
above.

**Force-cancel** (`POST /v1/admin/bookings/{id}/force-cancel`,
`bookings.force_cancel`) is the operator override of the participant-only
`cancel`: `{reason}` is required, `{refund}` optional. The admin module owns none
of the cancellation logic — it validates the state off the detail it just read
(409 `invalid_state` unless `pending_payment` / `confirmed`) and delegates
through the **`admin.BookingModerator`** port, which `*bookings.Service`
satisfies (`AdminForceCancel`). Inside bookings, `Cancel`, the teacher no-show
and the admin override all funnel through one private `doCancel`: write the
cancellation (now including `cancelled_by`), refund when asked, drop the pending
reminders, mail the participants. Nothing is duplicated, so nothing can drift.
The port carries no bookings-domain error, so `admin` never imports `bookings`.

**Disputes** (`internal/disputes`). A dispute is raised **by a participant** on a
`confirmed` or `completed` lesson and resolved **by an operator**. `status` goes
`open -> resolved | rejected` (terminal). A booking may hold at most one OPEN
dispute, enforced by the partial unique index
`disputes_one_open_per_booking (booking_id) WHERE status = 'open'`: the
repository inserts and maps SQLSTATE `23505` to 409 `dispute_exists` — race-safe,
never a check-then-insert, the same pattern as `payment_events` and
`reviews_booking_uniq`. Closed disputes are exempt, so a booking can be disputed
again and keeps its whole thread. `resolve` is a status-guarded `UPDATE ... WHERE
status = 'open'`; no rows back means the row is either unknown (404
`dispute_not_found`) or already closed (409 `already_resolved`), told apart by a
second read rather than by clobbering another operator's resolution.

Module boundaries, all following the existing rules:

| direction | how |
| --- | --- |
| participant routes need the booking's parties + status | `disputes` reads the booking context with its own SQL (`disputes.sql`), exactly as `reviews` does; the routes mount on the `/v1/bookings` group via `disputes.RegisterBookingRoutes` |
| operator routes | `disputes.RegisterAdminRoutes` mounts on `/v1/admin` behind `rbacGuard.Require(disputes.resolve)`, like `rbac.RegisterAdminRoutes` |
| a booking DTO carrying `can_raise_dispute` / `open_dispute` | `bookings.DisputeReader` (mirrors `ReviewReader`) — bookings defines it, `disputes.NewBookingGateway` implements it, `cmd/api` injects it with `SetDisputeReader`. Nil-safe. **bookings never imports disputes** |
| refunding a resolved dispute | `disputes.Refunder` — the consuming module defines it, `*bookings.Service.AdminRefund` satisfies it, `cmd/api` injects it with `SetRefunder`. It refunds the payment and deliberately leaves the booking's lifecycle alone (use force-cancel to also cancel the lesson) |
| the booking detail's dispute thread on the admin surface | `admin.sql` reads the `disputes` table directly — it's an ops tool (see above), so it does not route through the disputes service |

A failed refund on a resolution is logged, not returned: the operator's decision
is already committed and must not be lost to a provider hiccup — the same rule
the refund-on-cancel path follows. `TODO(payments): enqueue a refund retry.`

**Seed (phase D)**: three demo bookings — one `completed` a week ago carrying an
**open dispute** filed by the student, one `confirmed` in three days, one
`pending_payment` — so `/v1/admin/bookings` and `/v1/admin/disputes` are both
non-empty on a fresh database. The teardown clears `disputes` first (its
`booking_id` cascades, but `raised_by` / `resolved_by` reference `users`, which
does not).

## Tests

```bash
make test    # go test ./...
make vet
```

`internal/auth`, `internal/teachers`, `internal/availability`,
`internal/bookings`, `internal/payments`, `internal/reviews`, `internal/rbac`,
`internal/admin`, `internal/disputes`, `internal/email`, `internal/lessons` and
`cmd/worker` have unit tests (fakes + httptest);
`internal/bookings` also
has a slot-generation table test covering the tricky timezone / weekday /
midnight-wrap cases. The payments tests cover authorize-ok, decline (402),
double-pay (409), complete-too-early (409), complete → ledger,
cancel-with-refund → ledger reversed, webhook idempotency (same `event_id` twice
= one effect), and the earnings summary math. The phase-5 tests cover the
meeting-link visibility rule (hidden pre-payment / from non-participants, shown
to a participant of a confirmed booking), no-show student → completed + capture,
no-show teacher → cancelled + refund, no-show too-early → 409, the reminder
scheduler + notifier being called on `pay` / `cancel` / teacher no-show, nil
port safety, the email backend selection + templates, and the worker handler
(skips a cancelled booking, mails both parties for a confirmed one, retries on a
send error). The phase-6 review tests cover review-ok → 201 + aggregate bumped,
non-student → 403, not-completed → 409, duplicate → 409 (`23505` sentinel),
rating out of range → 400, comment too long → 400, list newest-first +
pagination + page-size cap + unknown slug 404, and the `can_review` / embedded
`review` DTO transitions for both participants. No DB is required for the test
suite.

## Not done yet

- Email verification, password reset, and rate-limiting on the auth endpoints.
- Changing a profile's `slug` (immutable for now), and clearing a trial price
  via `PATCH` (an omitted `trial_price_minor` is left unchanged; there is no way
  yet to express "remove the trial").
- Payments use a fake in-process provider — no real Payme / Click / Uzum
  adapter yet (the `payments.Provider` port is ready for one). No real webhook
  signing key rotation, no refund-retry queue (a failed refund on cancel is
  logged, not retried), and the `payout_ledger` has no clearing window or payout
  run (`held` → `available` is instant).
- Any cancellation window / penalty rules. A no-show is still filed by the
  teacher alone, but either participant can now dispute the lesson afterwards
  (phase D) and an operator resolves it.
- Disputes have no message thread of their own (one `reason` in, one
  `resolution` out — no back-and-forth), no attachments, no notification when
  one is raised or closed, and no auto-expiry of a dispute nobody triages.
  Resolving with `refund: true` refunds in full — there is no partial refund.
- Reviews cannot be edited or deleted, there is no teacher reply, and the
  `teachers.rating` aggregate is only ever nudged forward (a deleted review would
  not un-nudge it). The seeded per-teacher `rating` / `review_count` stay the
  historical baseline — the sample review rows are display-only.
- Reminders are enqueued from the synchronous `pay` request path; a real async
  payment provider would enqueue them from the `payment.authorized` webhook
  instead. There is no reschedule endpoint yet (only `Cancel` on cancel /
  teacher no-show).
- Email has no retry queue of its own (lifecycle mail is best-effort + logged;
  only the worker's reminder task retries, via asynq). No bounce handling, no
  per-user email preferences, no student-account timezone (student mail borrows
  the teacher's).
- `cmd/migrate` pulls in golang-migrate's transitive test deps (dktest/docker)
  as indirect modules — a known cost of using it as a library.
