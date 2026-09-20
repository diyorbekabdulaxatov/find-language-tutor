/**
 * Bookings data-access (browser). Slots is public; everything else needs auth.
 * All calls go through the generated `browserApi` (Bearer + cookie + 401
 * refresh-retry); the mappers turn the snake_case wire shapes into camelCase
 * view-models.
 */

import { browserApi } from "@/features/auth/browser-client";
import type { components } from "@/lib/api/schema";
import { mediaUrl } from "@/lib/media";
import type { Money } from "@/types/teacher";
import type {
  ResourceKind,
  ResourceStatus,
  ResourceType,
  SubmissionStatus,
} from "@/features/resources/types";

export type BookingStatus = components["schemas"]["BookingStatus"];

export const DURATION_OPTIONS = [30, 45, 60, 90, 120] as const;
export type Duration = (typeof DURATION_OPTIONS)[number];

/** The lengths a teacher with no lesson types can still be booked for. */
export const LEGACY_DURATIONS = [30, 60, 90, 120] as const;

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

/** One resource attached to a booking (summary only — no quiz content; see
 *  `listBookingAttachments` in `@/features/resources/api` for the full view). */
export interface BookingResource {
  id: string;
  resourceId: string;
  kind: ResourceKind;
  position: number;
  dueAt: string | null;
  type: ResourceType;
  title: string;
  resourceStatus: ResourceStatus;
  /** The viewer's own submission status; "" when not the student or not started. */
  submissionStatus: SubmissionStatus;
}

export type CancellationOutcome = "refunded" | "forfeited" | "unpaid";

export interface Booking {
  id: string;
  status: BookingStatus;
  startAt: string;
  endAt: string;
  durationMinutes: number;
  isTrial: boolean;
  /** the offering booked, or null for a booking made before lesson types */
  lessonType: { id: string; title: string } | null;
  price: Money;
  createdAt: string;
  cancelledAt: string | null;
  cancellationReason?: string;
  /** what happened to the money on cancellation; null while not cancelled */
  cancellationOutcome: CancellationOutcome | null;
  /** the free-cancellation deadline; null once completed or cancelled */
  cancellationPolicy: { freeCancelUntil: string; late: boolean } | null;
  /** how many times the student moved the lesson (max 3) */
  rescheduleCount: number;
  /** the start it was last moved away from; null if never moved */
  rescheduledFrom: string | null;
  /** true when POST /reschedule would be accepted for the viewer right now */
  canReschedule: boolean;
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
  /** attached materials / homework, summary only. Always an array. */
  resources: BookingResource[];
  teacher: {
    slug: string;
    displayName: string;
    timezone: string;
    avatarUrl: string;
  };
  student: { id: string; displayName: string };
}

/** Simulated payment methods the fake provider recognises. */
/** Labels live in the `bookings.testCard*` messages. */
export const TEST_METHODS = [
  { token: "pm_ok", hint: "•••• 4242" },
  { token: "pm_decline", hint: "•••• 0002" },
] as const;
export type MethodToken = (typeof TEST_METHODS)[number]["token"];

export interface EarningsSummary {
  totalEarned: Money;
  held: Money;
  available: Money;
  paid: Money;
  lessons: {
    /** Set for a lesson-booking row; null for a course-sale row (phase C3). */
    bookingId: string | null;
    /** Set for a course-sale row; null for a lesson-booking row. */
    courseEnrollmentId: string | null;
    /** Non-null only for a course-sale row — the simplest row-kind discriminator. */
    courseTitle: string | null;
    studentDisplayName: string;
    startAt: string;
    amount: Money;
    state: "held" | "available" | "paid" | "reversed";
    /** When a `held` lesson's clearing window closes; past for every other state. */
    availableAt: string;
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

function toBookingResource(r: components["schemas"]["BookingResource"]): BookingResource {
  return {
    id: r.id,
    resourceId: r.resource_id,
    kind: r.kind,
    position: r.position,
    dueAt: r.due_at,
    type: r.type,
    title: r.title,
    resourceStatus: r.resource_status,
    submissionStatus: r.submission_status,
  };
}

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
    lessonType: b.lesson_type
      ? { id: b.lesson_type.id, title: b.lesson_type.title }
      : null,
    price: money(b.price),
    createdAt: b.created_at,
    cancelledAt: b.cancelled_at,
    cancellationReason: b.cancellation_reason,
    cancellationOutcome: b.cancellation_outcome ?? null,
    cancellationPolicy: b.cancellation_policy
      ? {
          freeCancelUntil: b.cancellation_policy.free_cancel_until,
          late: b.cancellation_policy.late,
        }
      : null,
    rescheduleCount: b.reschedule_count ?? 0,
    rescheduledFrom: b.rescheduled_from ?? null,
    canReschedule: b.can_reschedule ?? false,
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
    resources: (b.resources ?? []).map(toBookingResource),
    teacher: {
      slug: b.teacher.slug,
      displayName: b.teacher.display_name,
      timezone: b.teacher.timezone,
      avatarUrl: mediaUrl(b.teacher.avatar_url),
    },
    student: { id: b.student.id, displayName: b.student.display_name },
  };
}

export async function getSlots(
  slug: string,
  opts: { from?: string; to?: string; duration: Duration; lessonTypeId?: string },
): Promise<SlotsResult> {
  const { data, error, response } = await browserApi.GET(
    "/v1/teachers/{slug}/slots",
    {
      params: {
        path: { slug },
        query: {
          from: opts.from,
          to: opts.to,
          duration: opts.duration,
          lesson_type_id: opts.lessonTypeId,
        },
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

/** One trial per student per teacher — what the checkout asks before offering the trial card. */
export type TrialEligibility =
  | { eligible: true }
  | { eligible: false; reason: "already_booked"; bookingId: string }
  | { eligible: false; reason: "own_profile" };

export async function getTrialEligibility(slug: string): Promise<TrialEligibility> {
  const { data, error, response } = await browserApi.GET(
    "/v1/teachers/{slug}/trial-eligibility",
    { params: { path: { slug } } },
  );
  if (error || !data) {
    throw toError(error, response.status, "Could not check trial eligibility.");
  }
  if (data.eligible) return { eligible: true };
  if (data.reason === "already_booked" && data.booking_id) {
    return { eligible: false, reason: "already_booked", bookingId: data.booking_id };
  }
  return { eligible: false, reason: "own_profile" };
}

export async function createBooking(input: {
  teacherSlug: string;
  startAt: string;
  durationMinutes: Duration;
  isTrial?: boolean;
  /** the offering the student picked; it sets the price and the trial flag */
  lessonTypeId?: string;
}): Promise<Booking> {
  const { data, error, response } = await browserApi.POST("/v1/bookings", {
    body: {
      teacher_slug: input.teacherSlug,
      start_at: input.startAt,
      duration_minutes: input.durationMinutes,
      is_trial: input.isTrial ?? false,
      lesson_type_id: input.lessonTypeId,
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
    paid: { amountMinor: data.paid_minor, currency: cur },
    lessons: data.lessons.map((l) => ({
      bookingId: l.booking_id ?? null,
      courseEnrollmentId: l.course_enrollment_id ?? null,
      courseTitle: l.course_title ?? null,
      studentDisplayName: l.student_display_name,
      startAt: l.start_at,
      amount: { amountMinor: l.amount_minor, currency: cur },
      state: l.state,
      availableAt: l.available_at,
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

/**
 * Cancel a booking. A student cancelling after the free-cancellation deadline
 * must pass acknowledgeForfeit, or the backend refuses with 409
 * `late_cancellation` and nothing changes.
 */
export async function cancelBooking(
  id: string,
  opts: { reason?: string; acknowledgeForfeit?: boolean } = {},
): Promise<Booking> {
  const { data, error, response } = await browserApi.POST(
    "/v1/bookings/{id}/cancel",
    {
      params: { path: { id } },
      body: {
        ...(opts.reason ? { reason: opts.reason } : {}),
        acknowledge_forfeit: opts.acknowledgeForfeit ?? false,
      },
    },
  );
  if (error || !data) {
    throw toError(error, response.status, "Could not cancel the booking.");
  }
  return toBooking(data);
}

/** Move a lesson to another bookable start of the same teacher (student-only). */
export async function rescheduleBooking(id: string, startAt: string): Promise<Booking> {
  const { data, error, response } = await browserApi.POST(
    "/v1/bookings/{id}/reschedule",
    { params: { path: { id } }, body: { start_at: startAt } },
  );
  if (error || !data) {
    throw toError(error, response.status, "Could not move the lesson.");
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
