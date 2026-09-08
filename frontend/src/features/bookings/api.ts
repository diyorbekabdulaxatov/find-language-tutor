/**
 * Bookings data-access (browser). Slots is public; everything else needs auth.
 * All calls go through the generated `browserApi` (Bearer + cookie + 401
 * refresh-retry); the mappers turn the snake_case wire shapes into camelCase
 * view-models.
 */

import { authedFetch, browserApi } from "@/features/auth/browser-client";
import type { components } from "@/lib/api/schema";
import type { Money } from "@/types/teacher";

const baseUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

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

/** The generated Booking schema won't carry `payment` until the backend adds it. */
type WireBookingMaybePayment = WireBooking & {
  payment?: { status: PaymentStatus; amount: WireMoney } | null;
};

function toBooking(raw: WireBooking): Booking {
  const b = raw as WireBookingMaybePayment;
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
    payment: b.payment
      ? { status: b.payment.status, amount: money(b.payment.amount) }
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

/* -------------------------------------------------------------------------- */
/* Phase 4 — payments. Hand-typed over authedFetch until these land in         */
/* openapi.yaml; swap to browserApi after `npm run gen:api`.                   */
/* -------------------------------------------------------------------------- */

async function raw<T>(
  path: string,
  init: RequestInit,
  fallback: string,
): Promise<T> {
  const res = await authedFetch(`${baseUrl}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init.headers },
  });
  const body =
    res.status === 204 ? undefined : await res.json().catch(() => undefined);
  if (!res.ok) {
    const e = body as ErrorBody | undefined;
    throw new BookingError(
      e?.error?.message ?? fallback,
      e?.error?.code ?? "unknown",
      res.status,
    );
  }
  return body as T;
}

/** Pay for a pending booking — authorizes the hold and confirms the lesson. */
export async function payBooking(
  id: string,
  methodToken: MethodToken,
): Promise<Booking> {
  const b = await raw<WireBooking>(
    `/v1/bookings/${encodeURIComponent(id)}/pay`,
    { method: "POST", body: JSON.stringify({ method_token: methodToken }) },
    "Payment could not be processed.",
  );
  return toBooking(b);
}

/** Teacher marks a past confirmed lesson complete — captures the payment. */
export async function completeBooking(id: string): Promise<Booking> {
  const b = await raw<WireBooking>(
    `/v1/bookings/${encodeURIComponent(id)}/complete`,
    { method: "POST", body: "{}" },
    "Could not mark the lesson complete.",
  );
  return toBooking(b);
}

interface WireEarnings {
  total_earned_minor: number;
  held_minor: number;
  available_minor: number;
  currency: Money["currency"];
  lessons: {
    booking_id: string;
    student_display_name: string;
    start_at: string;
    amount_minor: number;
    state: "held" | "available" | "reversed";
  }[];
}

export async function getEarnings(): Promise<EarningsSummary> {
  const w = await raw<WireEarnings>(
    "/v1/teachers/me/earnings",
    { method: "GET" },
    "Could not load your earnings.",
  );
  const cur = w.currency;
  return {
    totalEarned: { amountMinor: w.total_earned_minor, currency: cur },
    held: { amountMinor: w.held_minor, currency: cur },
    available: { amountMinor: w.available_minor, currency: cur },
    lessons: w.lessons.map((l) => ({
      bookingId: l.booking_id,
      studentDisplayName: l.student_display_name,
      startAt: l.start_at,
      amount: { amountMinor: l.amount_minor, currency: cur },
      state: l.state,
    })),
  };
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
