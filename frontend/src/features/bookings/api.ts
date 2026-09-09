/**
 * Bookings data-access (browser). Slots is public; everything else needs auth.
 * All calls go through the generated `browserApi` (Bearer + cookie + 401
 * refresh-retry); the mappers turn the snake_case wire shapes into camelCase
 * view-models.
 */

import { browserApi } from "@/features/auth/browser-client";
import type { components } from "@/lib/api/schema";
import type { Money } from "@/types/teacher";

export type BookingStatus = components["schemas"]["BookingStatus"];

export const DURATION_OPTIONS = [30, 60, 90, 120] as const;
export type Duration = (typeof DURATION_OPTIONS)[number];

export interface BookableSlot {
  /** RFC3339 UTC */
  startAt: string;
  endAt: string;
  price: Money;
}

export interface SlotsResult {
  teacherSlug: string;
  timezone: string;
  from: string;
  to: string;
  durationMinutes: number;
  slots: BookableSlot[];
}

export type PaymentStatus =
  | "requires_payment"
  | "authorized"
  | "captured"
  | "refunded"
  | "failed";

export interface PaymentInfo {
  status: PaymentStatus;
  amount: Money;
}

export type NoShowParty = "" | "student" | "teacher";

export type DisputeStatus = components["schemas"]["DisputeStatus"];

export interface OpenDispute {
  id: string;
  status: DisputeStatus;
  reason: string;
  createdAt: string;
}

export interface Dispute {
  id: string;
  bookingId: string;
  status: DisputeStatus;
  reason: string;
  resolution: string;
  raisedBy: { id: string; displayName: string };
  resolvedBy: { id: string; displayName: string } | null;
  createdAt: string;
  resolvedAt: string | null;
}

export interface Booking {
  id: string;
  status: BookingStatus;
  startAt: string;
  endAt: string;
  durationMinutes: number;
  isTrial: boolean;
  price: Money;
  createdAt: string;
  cancelledAt: string | null;
  cancellationReason?: string;
  payment: PaymentInfo | null;
  /** Effective video link. Only populated for a participant once confirmed. */
  meetingUrl: string;
  noShowParty: NoShowParty;
  /** true when the viewer is the student, status is completed, and no review yet. */
  canReview: boolean;
  /** the caller's review of this booking, if left. */
  review: { rating: number; comment: string; createdAt: string } | null;
  /** true when a participant may open a dispute (confirmed/completed, none open). */
  canRaiseDispute: boolean;
  /** the booking's open dispute, if any (visible to both participants). */
  openDispute: OpenDispute | null;
  teacher: {
    slug: string;
    displayName: string;
    timezone: string;
    avatarUrl: string;
  };
  student: { id: string; displayName: string };
}

/** Simulated payment methods the fake provider recognises. */
export const TEST_METHODS = [
  { token: "pm_ok", label: "Test card — succeeds", hint: "•••• 4242" },
  { token: "pm_decline", label: "Test card — declined", hint: "•••• 0002" },
] as const;
export type MethodToken = (typeof TEST_METHODS)[number]["token"];

export interface EarningsSummary {
  totalEarned: Money;
  held: Money;
  available: Money;
  lessons: {
    bookingId: string;
    studentDisplayName: string;
    startAt: string;
    amount: Money;
    state: "held" | "available" | "reversed";
  }[];
}

export class BookingError extends Error {
  code: string;
  status: number;
  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "BookingError";
    this.code = code;
    this.status = status;
  }
}

type ErrorBody = { error?: { code?: string; message?: string } };

function toError(error: unknown, status: number, fallback: string): BookingError {
  const b = error as ErrorBody | undefined;
  return new BookingError(
    b?.error?.message ?? fallback,
    b?.error?.code ?? "unknown",
    status,
  );
}

/* ---- wire -> view-model ---- */

type WireMoney = components["schemas"]["Money"];
type WireSlot = components["schemas"]["Slot"];
type WireBooking = components["schemas"]["Booking"];

const money = (m: WireMoney): Money => ({
  amountMinor: m.amount_minor,
  currency: m.currency,
});

function toBooking(b: WireBooking): Booking {
  // `payment` / `meeting_url` are absent from list responses; read defensively.
  const wp = (b as { payment?: components["schemas"]["BookingPayment"] | null })
    .payment;
  return {
    id: b.id,
    status: b.status,
    startAt: b.start_at,
    endAt: b.end_at,
    durationMinutes: b.duration_minutes,
    isTrial: b.is_trial,
    price: money(b.price),
    createdAt: b.created_at,
    cancelledAt: b.cancelled_at,
    cancellationReason: b.cancellation_reason,
    payment: wp
      ? {
          status: wp.status,
          amount: {
            amountMinor: wp.amount_minor,
            currency: wp.currency as Money["currency"],
          },
        }
      : null,
    meetingUrl: b.meeting_url ?? "",
    noShowParty: b.no_show_party ?? "",
    canReview: b.can_review ?? false,
    review: b.review
      ? {
          rating: b.review.rating,
          comment: b.review.comment,
          createdAt: b.review.created_at,
        }
      : null,
    canRaiseDispute: b.can_raise_dispute ?? false,
    openDispute: b.open_dispute
      ? {
          id: b.open_dispute.id,
          status: b.open_dispute.status,
          reason: b.open_dispute.reason,
          createdAt: b.open_dispute.created_at,
        }
      : null,
    teacher: {
      slug: b.teacher.slug,
      displayName: b.teacher.display_name,
      timezone: b.teacher.timezone,
      avatarUrl: b.teacher.avatar_url,
    },
    student: { id: b.student.id, displayName: b.student.display_name },
  };
}

export async function getSlots(
  slug: string,
  opts: { from?: string; to?: string; duration: Duration },
): Promise<SlotsResult> {
  const { data, error, response } = await browserApi.GET(
    "/v1/teachers/{slug}/slots",
    {
      params: {
        path: { slug },
        query: { from: opts.from, to: opts.to, duration: opts.duration },
      },
    },
  );
  if (error || !data) {
    throw toError(error, response.status, "Could not load available times.");
  }
  return {
    teacherSlug: data.teacher_slug,
    timezone: data.timezone,
    from: data.from,
    to: data.to,
    durationMinutes: data.duration_minutes,
    slots: data.slots.map((s: WireSlot) => ({
      startAt: s.start_at,
      endAt: s.end_at,
      price: money(s.price),
    })),
  };
}

export async function createBooking(input: {
  teacherSlug: string;
  startAt: string;
  durationMinutes: Duration;
  isTrial?: boolean;
}): Promise<Booking> {
  const { data, error, response } = await browserApi.POST("/v1/bookings", {
    body: {
      teacher_slug: input.teacherSlug,
      start_at: input.startAt,
      duration_minutes: input.durationMinutes,
      is_trial: input.isTrial ?? false,
    },
  });
  if (error || !data) {
    throw toError(error, response.status, "Could not create the booking.");
  }
  return toBooking(data);
}

export async function listBookings(
  role?: "student" | "teacher",
): Promise<Booking[]> {
  const { data, error, response } = await browserApi.GET("/v1/bookings", {
    params: { query: role ? { role } : {} },
  });
  if (error || !data) {
    throw toError(error, response.status, "Could not load your bookings.");
  }
  return data.bookings.map(toBooking);
}

export async function getBooking(id: string): Promise<Booking> {
  const { data, error, response } = await browserApi.GET("/v1/bookings/{id}", {
    params: { path: { id } },
  });
  if (error || !data) {
    throw toError(error, response.status, "Could not load that booking.");
  }
  return toBooking(data);
}

/* ---- Phase 4 — payments ---- */

/** Pay for a pending booking — authorizes the hold and confirms the lesson. */
export async function payBooking(
  id: string,
  methodToken: MethodToken,
): Promise<Booking> {
  const { data, error, response } = await browserApi.POST(
    "/v1/bookings/{id}/pay",
    { params: { path: { id } }, body: { method_token: methodToken } },
  );
  if (error || !data) {
    throw toError(error, response.status, "Payment could not be processed.");
  }
  return toBooking(data);
}

/** Teacher marks a past confirmed lesson complete — captures the payment. */
export async function completeBooking(id: string): Promise<Booking> {
  const { data, error, response } = await browserApi.POST(
    "/v1/bookings/{id}/complete",
    { params: { path: { id } } },
  );
  if (error || !data) {
    throw toError(error, response.status, "Could not mark the lesson complete.");
  }
  return toBooking(data);
}

export async function getEarnings(): Promise<EarningsSummary> {
  const { data, error, response } = await browserApi.GET("/v1/payments/me", {});
  if (error || !data) {
    throw toError(error, response.status, "Could not load your earnings.");
  }
  const cur = data.currency as Money["currency"];
  return {
    totalEarned: { amountMinor: data.total_earned_minor, currency: cur },
    held: { amountMinor: data.held_minor, currency: cur },
    available: { amountMinor: data.available_minor, currency: cur },
    lessons: data.lessons.map((l) => ({
      bookingId: l.booking_id,
      studentDisplayName: l.student_display_name,
      startAt: l.start_at,
      amount: { amountMinor: l.amount_minor, currency: cur },
      state: l.state,
    })),
  };
}

/* ---- Phase 5 — lessons ---- */

/** Teacher sets (or clears, with "") the per-booking meeting link. */
export async function setMeetingLink(id: string, url: string): Promise<Booking> {
  const { data, error, response } = await browserApi.PUT(
    "/v1/bookings/{id}/meeting-link",
    { params: { path: { id } }, body: { url } },
  );
  if (error || !data) {
    throw toError(error, response.status, "Could not update the meeting link.");
  }
  return toBooking(data);
}

/** Teacher reports a no-show for a lesson that has started. */
export async function reportNoShow(
  id: string,
  party: "student" | "teacher",
): Promise<Booking> {
  const { data, error, response } = await browserApi.POST(
    "/v1/bookings/{id}/no-show",
    { params: { path: { id } }, body: { party } },
  );
  if (error || !data) {
    throw toError(error, response.status, "Could not report the no-show.");
  }
  return toBooking(data);
}

export async function cancelBooking(
  id: string,
  reason?: string,
): Promise<Booking> {
  const { data, error, response } = await browserApi.POST(
    "/v1/bookings/{id}/cancel",
    { params: { path: { id } }, body: reason ? { reason } : {} },
  );
  if (error || !data) {
    throw toError(error, response.status, "Could not cancel the booking.");
  }
  return toBooking(data);
}

/* -------------------------------- disputes ------------------------------- */

function toDispute(d: components["schemas"]["Dispute"]): Dispute {
  return {
    id: d.id,
    bookingId: d.booking_id,
    status: d.status,
    reason: d.reason,
    resolution: d.resolution,
    raisedBy: { id: d.raised_by.id, displayName: d.raised_by.display_name },
    resolvedBy: d.resolved_by
      ? { id: d.resolved_by.id, displayName: d.resolved_by.display_name }
      : null,
    createdAt: d.created_at,
    resolvedAt: d.resolved_at,
  };
}

/** A participant opens a dispute on a confirmed / completed lesson. */
export async function raiseDispute(id: string, reason: string): Promise<Dispute> {
  const { data, error, response } = await browserApi.POST(
    "/v1/bookings/{id}/disputes",
    { params: { path: { id } }, body: { reason } },
  );
  if (error || !data) {
    throw toError(error, response.status, "Could not open the dispute.");
  }
  return toDispute(data);
}

/** The booking's dispute thread, newest first. Participant-only. */
export async function getBookingDisputes(id: string): Promise<Dispute[]> {
  const { data, error, response } = await browserApi.GET(
    "/v1/bookings/{id}/disputes",
    { params: { path: { id } } },
  );
  if (error || !data) {
    throw toError(error, response.status, "Could not load the disputes.");
  }
  return data.disputes.map(toDispute);
}
