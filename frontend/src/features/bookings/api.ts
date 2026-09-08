/**
 * Bookings data-access (browser). Slots is public; everything else needs auth.
 *
 * NOTE: `POST /v1/bookings`, `GET /v1/bookings`, etc. are being added to the
 * backend (Phase 3). Until they land in openapi.yaml + `npm run gen:api`, this
 * uses `authedFetch` with hand-written wire types that match the agreed
 * contract; swap to the typed `browserApi` once the schema regenerates.
 */

import { authedFetch } from "@/features/auth/browser-client";
import type { Money } from "@/types/teacher";

const baseUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export type BookingStatus =
  | "pending_payment"
  | "confirmed"
  | "completed"
  | "cancelled";

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
  teacher: {
    slug: string;
    displayName: string;
    timezone: string;
    avatarUrl: string;
  };
  student: { id: string; displayName: string };
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

/* ---- wire (snake_case) shapes ---- */

interface WireMoney {
  amount_minor: number;
  currency: Money["currency"];
}
interface WireSlot {
  start_at: string;
  end_at: string;
  price: WireMoney;
}
interface WireSlots {
  teacher_slug: string;
  timezone: string;
  from: string;
  to: string;
  duration_minutes: number;
  slots: WireSlot[];
}
interface WireBooking {
  id: string;
  status: BookingStatus;
  start_at: string;
  end_at: string;
  duration_minutes: number;
  is_trial: boolean;
  price: WireMoney;
  created_at: string;
  cancelled_at: string | null;
  cancellation_reason?: string;
  teacher: {
    slug: string;
    display_name: string;
    timezone: string;
    avatar_url: string;
  };
  student: { id: string; display_name: string };
}

const money = (m: WireMoney): Money => ({
  amountMinor: m.amount_minor,
  currency: m.currency,
});

function toBooking(b: WireBooking): Booking {
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
    teacher: {
      slug: b.teacher.slug,
      displayName: b.teacher.display_name,
      timezone: b.teacher.timezone,
      avatarUrl: b.teacher.avatar_url,
    },
    student: { id: b.student.id, displayName: b.student.display_name },
  };
}

async function call<T>(
  path: string,
  init: RequestInit,
  fallback: string,
): Promise<T> {
  const res = await authedFetch(`${baseUrl}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init.headers },
  });
  const body = res.status === 204 ? undefined : await res.json().catch(() => undefined);
  if (!res.ok) {
    const err = body as { error?: { code?: string; message?: string } } | undefined;
    throw new BookingError(
      err?.error?.message ?? fallback,
      err?.error?.code ?? "unknown",
      res.status,
    );
  }
  return body as T;
}

export async function getSlots(
  slug: string,
  opts: { from?: string; to?: string; duration: number },
): Promise<SlotsResult> {
  const q = new URLSearchParams({ duration: String(opts.duration) });
  if (opts.from) q.set("from", opts.from);
  if (opts.to) q.set("to", opts.to);
  const w = await call<WireSlots>(
    `/v1/teachers/${encodeURIComponent(slug)}/slots?${q}`,
    { method: "GET" },
    "Could not load available times.",
  );
  return {
    teacherSlug: w.teacher_slug,
    timezone: w.timezone,
    from: w.from,
    to: w.to,
    durationMinutes: w.duration_minutes,
    slots: w.slots.map((s) => ({
      startAt: s.start_at,
      endAt: s.end_at,
      price: money(s.price),
    })),
  };
}

export async function createBooking(input: {
  teacherSlug: string;
  startAt: string;
  durationMinutes: number;
  isTrial?: boolean;
}): Promise<Booking> {
  const w = await call<WireBooking>(
    "/v1/bookings",
    {
      method: "POST",
      body: JSON.stringify({
        teacher_slug: input.teacherSlug,
        start_at: input.startAt,
        duration_minutes: input.durationMinutes,
        is_trial: input.isTrial ?? false,
      }),
    },
    "Could not create the booking.",
  );
  return toBooking(w);
}

export async function listBookings(
  role?: "student" | "teacher",
): Promise<Booking[]> {
  const q = role ? `?role=${role}` : "";
  const w = await call<{ bookings: WireBooking[] }>(
    `/v1/bookings${q}`,
    { method: "GET" },
    "Could not load your bookings.",
  );
  return w.bookings.map(toBooking);
}

export async function getBooking(id: string): Promise<Booking> {
  const w = await call<WireBooking>(
    `/v1/bookings/${encodeURIComponent(id)}`,
    { method: "GET" },
    "Could not load that booking.",
  );
  return toBooking(w);
}

export async function confirmBooking(id: string): Promise<Booking> {
  const w = await call<WireBooking>(
    `/v1/bookings/${encodeURIComponent(id)}/confirm`,
    { method: "POST", body: "{}" },
    "Could not confirm the booking.",
  );
  return toBooking(w);
}

export async function cancelBooking(
  id: string,
  reason?: string,
): Promise<Booking> {
  const w = await call<WireBooking>(
    `/v1/bookings/${encodeURIComponent(id)}/cancel`,
    { method: "POST", body: JSON.stringify({ reason: reason ?? "" }) },
    "Could not cancel the booking.",
  );
  return toBooking(w);
}
