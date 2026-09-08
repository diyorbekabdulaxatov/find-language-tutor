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
| GET | `/v1/auth/me` | the signed-in user; needs `Authorization: Bearer` |
| PATCH | `/v1/auth/me` | edit own account (`display_name` only; email is read-only) |
| GET | `/v1/teachers` | `?language&kind&max_price_minor&q&sort&page&page_size` |
| POST | `/v1/teachers` | claim/create the caller's profile; Bearer token; 409 if they already own one |
| GET | `/v1/teachers/me` | the caller's own profile; Bearer token; 404 if not created yet |
| GET | `/v1/teachers/{slug}` | full profile, 404 if missing |
| PATCH | `/v1/teachers/{slug}` | edit own profile (partial); Bearer token, must own it; 403/404 otherwise. Now also accepts `meeting_url` (default video room; http(s) or empty) |
| GET | `/v1/teachers/{slug}/availability` | weekly recurring slots (UTC), 404 if missing |
| PUT | `/v1/teachers/{slug}/availability` | replace the full weekly set; Bearer token, must own the profile |
| GET | `/v1/teachers/{slug}/slots` | `?from&to&duration` — concrete bookable start times (UTC); public; 400 if the window > 21 days |
| POST | `/v1/bookings` | book a lesson; Bearer token; 201 `pending_payment` + opens a `requires_payment` intent; 409 `slot_unavailable` / `slot_taken` |
| GET | `/v1/bookings` | `?role=student\|teacher&status=` — the caller's bookings, newest first; Bearer token |
| GET | `/v1/bookings/{id}` | full booking (with embedded `payment`); Bearer token; 404 if missing, 403 if not a participant |
| POST | `/v1/bookings/{id}/pay` | `{method_token}`; student only; authorize → `pending_payment → confirmed`; 402 `payment_failed`, 409 `already_paid` |
| POST | `/v1/bookings/{id}/complete` | teacher-owner only; `confirmed → completed` + capture + payout-ledger row; 409 `too_early`, 502 `capture_failed` |
| POST | `/v1/bookings/{id}/cancel` | `{reason?}`; `pending_payment\|confirmed → cancelled`; participant only; refunds/voids the intent; cancels reminders + emails the other party; 409 if already done |
| PUT | `/v1/bookings/{id}/meeting-link` | `{url}`; teacher-owner only; sets the per-booking link override (empty clears it); 403/404 |
| POST | `/v1/bookings/{id}/no-show` | `{party}`; teacher-owner only; from `confirmed` once started (409 `too_early`); `student` → `completed` + capture, `teacher` → `cancelled` + refund |
| POST | `/v1/payments/webhook` | provider event; unauthenticated (signed); idempotent by `event_id`; 200 on a well-formed duplicate |
| GET | `/v1/payments/me` | the caller's teacher earnings summary; Bearer token; 404 if they own no profile |

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

## Tests

```bash
make test    # go test ./...
make vet
```

`internal/auth`, `internal/teachers`, `internal/availability`,
`internal/bookings`, `internal/payments`, `internal/email`, `internal/lessons`
and `cmd/worker` have unit tests (fakes + httptest); `internal/bookings` also
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
send error). No DB is required for the test suite.

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
- Any cancellation window / penalty rules (no-show has no dispute / appeal flow
  — the teacher's report is final for the MVP, and only the teacher can file it).
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
