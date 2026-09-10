/**
 * Admin data-access (browser). Every call is behind the RBAC guard on the
 * backend.
 *
 * The transport is `authedFetch` + a small `call()` helper (throws a typed
 * `AdminError` with `.code`). Response/request shapes for the newer endpoints
 * are pinned to the generated `components["schemas"]` types; the older
 * phase-A/B mappers still carry hand-written wire types.
 */

import { authedFetch } from "@/features/auth/browser-client";
import type { components } from "@/lib/api/schema";
import type { Money } from "@/types/teacher";

const baseUrl = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export class AdminError extends Error {
  code: string;
  status: number;
  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "AdminError";
    this.code = code;
    this.status = status;
  }
}

type ErrorBody = { error?: { code?: string; message?: string } };

async function call<T>(
  path: string,
  init: RequestInit = {},
  fallback = "Something went wrong.",
): Promise<T> {
  const res = await authedFetch(`${baseUrl}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init.headers },
  });
  const body =
    res.status === 204 ? undefined : await res.json().catch(() => undefined);
  if (!res.ok) {
    const e = body as ErrorBody | undefined;
    throw new AdminError(
      e?.error?.message ?? fallback,
      e?.error?.code ?? "unknown",
      res.status,
    );
  }
  return body as T;
}

/* -------------------------------- metrics -------------------------------- */

export interface AdminMetrics {
  usersTotal: number;
  teachersTotal: number;
  teachersPending: number;
  bookingsTotal: number;
  bookingsThisWeek: number;
  gmv: Money;
}

interface WireMetrics {
  users_total: number;
  teachers_total: number;
  teachers_pending: number;
  bookings_total: number;
  bookings_this_week: number;
  gmv_minor: number;
  gmv_currency: string;
}

export async function getMetrics(): Promise<AdminMetrics> {
  const w = await call<WireMetrics>("/v1/admin/metrics", {}, "Could not load metrics.");
  return {
    usersTotal: w.users_total,
    teachersTotal: w.teachers_total,
    teachersPending: w.teachers_pending,
    bookingsTotal: w.bookings_total,
    bookingsThisWeek: w.bookings_this_week,
    gmv: { amountMinor: w.gmv_minor, currency: w.gmv_currency as Money["currency"] },
  };
}

/* --------------------------------- users --------------------------------- */

export interface AdminUserRow {
  id: string;
  email: string;
  displayName: string;
  createdAt: string;
  isTeacher: boolean;
  bookingCount: number;
}

export interface Paged<T> {
  items: T[];
  total: number;
}

interface WireUserRow {
  id: string;
  email: string;
  display_name: string;
  created_at: string;
  is_teacher: boolean;
  booking_count: number;
}

const toUserRow = (u: WireUserRow): AdminUserRow => ({
  id: u.id,
  email: u.email,
  displayName: u.display_name,
  createdAt: u.created_at,
  isTeacher: u.is_teacher,
  bookingCount: u.booking_count,
});

export async function listUsers(opts: {
  q?: string;
  page?: number;
}): Promise<Paged<AdminUserRow>> {
  const p = new URLSearchParams({ page: String(opts.page ?? 1) });
  if (opts.q) p.set("q", opts.q);
  const w = await call<{ users: WireUserRow[]; total: number }>(
    `/v1/admin/users?${p}`,
    {},
    "Could not load users.",
  );
  return { items: w.users.map(toUserRow), total: w.total };
}

export interface AdminUserDetail {
  user: {
    id: string;
    email: string;
    displayName: string;
    createdAt: string;
  };
  roles: { id: string; name: string }[];
  teacherProfile: {
    slug: string;
    status: TeacherStatus;
    verified: boolean;
  } | null;
  bookings: {
    id: string;
    status: string;
    startAt: string;
    roleInBooking: "student" | "teacher";
    otherPartyName: string;
    price: Money;
  }[];
  paymentsSummary: {
    authorizedMinor: number;
    capturedMinor: number;
    refundedMinor: number;
    currency: string;
  };
}

export async function getUser(id: string): Promise<AdminUserDetail> {
  const w = await call<{
    user: {
      id: string;
      email: string;
      display_name: string;
      created_at: string;
    };
    roles?: { id: string; name: string }[];
    teacher_profile: {
      slug: string;
      status: TeacherStatus;
      verified: boolean;
    } | null;
    bookings: {
      id: string;
      status: string;
      start_at: string;
      role_in_booking: "student" | "teacher";
      other_party_name: string;
      price: { amount_minor: number; currency: string };
    }[];
    payments_summary: {
      authorized_minor: number;
      captured_minor: number;
      refunded_minor: number;
      currency: string;
    };
  }>(`/v1/admin/users/${encodeURIComponent(id)}`, {}, "Could not load that user.");

  return {
    user: {
      id: w.user.id,
      email: w.user.email,
      displayName: w.user.display_name,
      createdAt: w.user.created_at,
    },
    roles: w.roles ?? [],
    teacherProfile: w.teacher_profile,
    bookings: w.bookings.map((b) => ({
      id: b.id,
      status: b.status,
      startAt: b.start_at,
      roleInBooking: b.role_in_booking,
      otherPartyName: b.other_party_name,
      price: {
        amountMinor: b.price.amount_minor,
        currency: b.price.currency as Money["currency"],
      },
    })),
    paymentsSummary: {
      authorizedMinor: w.payments_summary.authorized_minor,
      capturedMinor: w.payments_summary.captured_minor,
      refundedMinor: w.payments_summary.refunded_minor,
      currency: w.payments_summary.currency,
    },
  };
}

/* ------------------------------ teachers (mod) --------------------------- */

export type TeacherStatus = "pending" | "approved" | "rejected" | "suspended";

export interface AdminTeacherRow {
  slug: string;
  displayName: string;
  status: TeacherStatus;
  verified: boolean;
  headline: string;
  countryName: string;
  createdAt: string;
  owner: { id: string; email: string };
}

interface WireTeacherRow {
  slug: string;
  display_name: string;
  status: TeacherStatus;
  verified: boolean;
  headline: string;
  country_name: string;
  created_at: string;
  owner: { id: string; email: string };
}

const toTeacherRow = (t: WireTeacherRow): AdminTeacherRow => ({
  slug: t.slug,
  displayName: t.display_name,
  status: t.status,
  verified: t.verified,
  headline: t.headline,
  countryName: t.country_name,
  createdAt: t.created_at,
  owner: t.owner,
});

export async function listTeachers(opts: {
  status?: TeacherStatus;
  q?: string;
  page?: number;
}): Promise<Paged<AdminTeacherRow>> {
  const p = new URLSearchParams({ page: String(opts.page ?? 1) });
  if (opts.status) p.set("status", opts.status);
  if (opts.q) p.set("q", opts.q);
  const w = await call<{ teachers: WireTeacherRow[]; total: number }>(
    `/v1/admin/teachers?${p}`,
    {},
    "Could not load teachers.",
  );
  return { items: w.teachers.map(toTeacherRow), total: w.total };
}

export interface AdminTeacherDetail {
  slug: string;
  displayName: string;
  headline: string;
  about: string;
  teachingStyle: string;
  city: string;
  countryName: string;
  timezone: string;
  pricePerHour: Money;
  status: TeacherStatus;
  verified: boolean;
  moderationNote: string;
  owner: { id: string; email: string; displayName: string };
}

export async function getTeacher(slug: string): Promise<AdminTeacherDetail> {
  const w = await call<{
    slug: string;
    display_name: string;
    headline: string;
    about: string;
    teaching_style: string;
    city: string;
    country_name: string;
    timezone: string;
    price_per_hour: { amount_minor: number; currency: string };
    status: TeacherStatus;
    verified: boolean;
    moderation_note: string;
    owner: { id: string; email: string; display_name: string };
  }>(`/v1/admin/teachers/${encodeURIComponent(slug)}`, {}, "Could not load that teacher.");
  return {
    slug: w.slug,
    displayName: w.display_name,
    headline: w.headline,
    about: w.about,
    teachingStyle: w.teaching_style,
    city: w.city,
    countryName: w.country_name,
    timezone: w.timezone,
    pricePerHour: {
      amountMinor: w.price_per_hour.amount_minor,
      currency: w.price_per_hour.currency as Money["currency"],
    },
    status: w.status,
    verified: w.verified,
    moderationNote: w.moderation_note,
    owner: {
      id: w.owner.id,
      email: w.owner.email,
      displayName: w.owner.display_name,
    },
  };
}

async function moderate(slug: string, action: string, body?: unknown) {
  await call(
    `/v1/admin/teachers/${encodeURIComponent(slug)}/${action}`,
    { method: "POST", body: body ? JSON.stringify(body) : "{}" },
    `Could not ${action} the teacher.`,
  );
}

export const approveTeacher = (slug: string) => moderate(slug, "approve");
export const rejectTeacher = (slug: string, note: string) =>
  moderate(slug, "reject", { note });
export const suspendTeacher = (slug: string, note: string) =>
  moderate(slug, "suspend", { note });
export const setTeacherVerified = (slug: string, verified: boolean) =>
  moderate(slug, "verify", { verified });

/* ------------------------------- RBAC roles ----------------------------- */

export interface PermissionInfo {
  key: string;
  description: string;
}

export interface Role {
  id: string;
  name: string;
  description: string;
  isSystem: boolean;
  permissions: string[];
  userCount: number;
}

interface WireRole {
  id: string;
  name: string;
  description: string;
  is_system: boolean;
  permissions: string[];
  user_count: number;
}

const toRole = (r: WireRole): Role => ({
  id: r.id,
  name: r.name,
  description: r.description,
  isSystem: r.is_system,
  permissions: r.permissions,
  userCount: r.user_count,
});

export async function getPermissionCatalog(): Promise<PermissionInfo[]> {
  const w = await call<{ key: string; description: string }[]>(
    "/v1/admin/permissions",
    {},
    "Could not load the permission catalog.",
  );
  return w;
}

export async function listRoles(): Promise<Role[]> {
  const w = await call<WireRole[]>("/v1/admin/roles", {}, "Could not load roles.");
  return w.map(toRole);
}

export async function createRole(input: {
  name: string;
  description: string;
  permissions: string[];
}): Promise<void> {
  await call(
    "/v1/admin/roles",
    { method: "POST", body: JSON.stringify(input) },
    "Could not create the role.",
  );
}

export async function updateRole(
  id: string,
  patch: { description?: string; permissions?: string[] },
): Promise<void> {
  await call(
    `/v1/admin/roles/${encodeURIComponent(id)}`,
    { method: "PATCH", body: JSON.stringify(patch) },
    "Could not update the role.",
  );
}

export async function deleteRole(id: string): Promise<void> {
  await call(
    `/v1/admin/roles/${encodeURIComponent(id)}`,
    { method: "DELETE" },
    "Could not delete the role.",
  );
}

export async function assignRole(userId: string, roleId: string): Promise<void> {
  await call(
    `/v1/admin/users/${encodeURIComponent(userId)}/roles`,
    { method: "POST", body: JSON.stringify({ role_id: roleId }) },
    "Could not assign the role.",
  );
}

export async function unassignRole(
  userId: string,
  roleId: string,
): Promise<void> {
  await call(
    `/v1/admin/users/${encodeURIComponent(userId)}/roles/${encodeURIComponent(roleId)}`,
    { method: "DELETE" },
    "Could not remove the role.",
  );
}

/* --------------------------- bookings (phase D) ------------------------- */

type WireBookingRow = components["schemas"]["AdminBookingRow"];
type WireBookingDetail = components["schemas"]["AdminBookingDetail"];
type WireBookingDispute = components["schemas"]["AdminBookingDispute"];

export type AdminBookingStatus = components["schemas"]["BookingStatus"];
export type AdminPaymentStatus = NonNullable<WireBookingRow["payment_status"]>;

export interface AdminBookingRow {
  id: string;
  status: AdminBookingStatus;
  startAt: string;
  endAt: string;
  teacher: { slug: string; displayName: string };
  student: { id: string; email: string; displayName: string };
  price: Money;
  paymentStatus: AdminPaymentStatus | null;
  createdAt: string;
  hasOpenDispute: boolean;
}

export interface AdminBookingDispute {
  id: string;
  status: components["schemas"]["DisputeStatus"];
  reason: string;
  resolution: string;
  raisedBy: { id: string; displayName: string };
  resolvedBy: { id: string; displayName: string } | null;
  createdAt: string;
  resolvedAt: string | null;
}

export interface AdminBookingDetail extends AdminBookingRow {
  durationMinutes: number;
  isTrial: boolean;
  payment: { status: AdminPaymentStatus; amount: Money } | null;
  meetingUrl: string;
  noShowParty: "" | "student" | "teacher";
  cancelledAt: string | null;
  cancelledBy: "" | "student" | "teacher" | "admin";
  cancellationReason: string;
  disputes: AdminBookingDispute[];
}

const toBookingRow = (b: WireBookingRow): AdminBookingRow => ({
  id: b.id,
  status: b.status,
  startAt: b.start_at,
  endAt: b.end_at,
  teacher: { slug: b.teacher.slug, displayName: b.teacher.display_name },
  student: {
    id: b.student.id,
    email: b.student.email,
    displayName: b.student.display_name,
  },
  price: { amountMinor: b.price.amount_minor, currency: b.price.currency as Money["currency"] },
  paymentStatus: b.payment_status ?? null,
  createdAt: b.created_at,
  hasOpenDispute: b.has_open_dispute,
});

const toBookingDispute = (d: WireBookingDispute): AdminBookingDispute => ({
  id: d.id,
  status: d.status,
  reason: d.reason,
  resolution: d.resolution,
  raisedBy: { id: d.raised_by.id, displayName: d.raised_by.display_name },
  resolvedBy: d.resolved_by
    ? { id: d.resolved_by.id, displayName: d.resolved_by.display_name }
    : null,
  createdAt: d.created_at,
  resolvedAt: d.resolved_at,
});

export async function listAdminBookings(opts: {
  status?: AdminBookingStatus;
  q?: string;
  page?: number;
}): Promise<Paged<AdminBookingRow>> {
  const p = new URLSearchParams({ page: String(opts.page ?? 1) });
  if (opts.status) p.set("status", opts.status);
  if (opts.q) p.set("q", opts.q);
  const w = await call<components["schemas"]["AdminBookingList"]>(
    `/v1/admin/bookings?${p}`,
    {},
    "Could not load bookings.",
  );
  return { items: w.bookings.map(toBookingRow), total: w.total };
}

export async function getAdminBooking(id: string): Promise<AdminBookingDetail> {
  const w = await call<WireBookingDetail>(
    `/v1/admin/bookings/${encodeURIComponent(id)}`,
    {},
    "Could not load that booking.",
  );
  return {
    ...toBookingRow(w),
    durationMinutes: w.duration_minutes,
    isTrial: w.is_trial,
    payment: w.payment
      ? {
          status: w.payment.status,
          amount: {
            amountMinor: w.payment.amount_minor,
            currency: w.payment.currency as Money["currency"],
          },
        }
      : null,
    meetingUrl: w.meeting_url,
    noShowParty: w.no_show_party,
    cancelledAt: w.cancelled_at,
    cancelledBy: w.cancelled_by,
    cancellationReason: w.cancellation_reason,
    disputes: w.disputes.map(toBookingDispute),
  };
}

export async function forceCancelBooking(
  id: string,
  reason: string,
  refund: boolean,
): Promise<void> {
  await call(
    `/v1/admin/bookings/${encodeURIComponent(id)}/force-cancel`,
    { method: "POST", body: JSON.stringify({ reason, refund }) },
    "Could not force-cancel the booking.",
  );
}

/* --------------------------- disputes (phase D) ------------------------- */

type WireAdminDisputeRow = components["schemas"]["AdminDisputeRow"];

export type DisputeQueueStatus = "open" | "resolved" | "rejected" | "all";

export interface AdminDisputeRow {
  id: string;
  bookingId: string;
  status: components["schemas"]["DisputeStatus"];
  reason: string;
  resolution: string;
  raisedBy: { id: string; displayName: string };
  resolvedBy: { id: string; displayName: string } | null;
  createdAt: string;
  resolvedAt: string | null;
  booking: {
    id: string;
    status: AdminBookingStatus;
    startAt: string;
    price: Money;
    teacher: { slug: string; displayName: string };
    student: { id: string; displayName: string };
  };
}

const toDisputeRow = (d: WireAdminDisputeRow): AdminDisputeRow => ({
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
  booking: {
    id: d.booking.id,
    status: d.booking.status,
    startAt: d.booking.start_at,
    price: {
      amountMinor: d.booking.price.amount_minor,
      currency: d.booking.price.currency as Money["currency"],
    },
    teacher: {
      slug: d.booking.teacher.slug,
      displayName: d.booking.teacher.display_name,
    },
    student: {
      id: d.booking.student.id,
      displayName: d.booking.student.display_name,
    },
  },
});

export async function listDisputes(opts: {
  status?: DisputeQueueStatus;
  page?: number;
}): Promise<Paged<AdminDisputeRow>> {
  const p = new URLSearchParams({ page: String(opts.page ?? 1) });
  if (opts.status) p.set("status", opts.status);
  const w = await call<components["schemas"]["AdminDisputeList"]>(
    `/v1/admin/disputes?${p}`,
    {},
    "Could not load the dispute queue.",
  );
  return { items: w.disputes.map(toDisputeRow), total: w.total };
}

export async function resolveDispute(
  id: string,
  body: { outcome: "resolved" | "rejected"; resolution: string; refund: boolean },
): Promise<void> {
  await call(
    `/v1/admin/disputes/${encodeURIComponent(id)}/resolve`,
    { method: "POST", body: JSON.stringify(body) },
    "Could not resolve the dispute.",
  );
}

/* --------------------------- payouts (phase E) ------------------------- */

type WirePayoutDashboard = components["schemas"]["AdminPayoutDashboard"];
type WirePayoutBatch = components["schemas"]["PayoutBatch"];
type WirePayoutBatchDetail = components["schemas"]["PayoutBatchDetail"];

export type PayoutBatchStatus = WirePayoutBatch["status"];

export interface OwedPayoutRow {
  teacher: { slug: string; displayName: string };
  available: Money;
  oldestAvailableAt: string;
}

export interface PayoutTotals {
  available: Money;
  held: Money;
  paid: Money;
}

export interface PayoutBatch {
  id: string;
  createdBy: { id: string; displayName: string };
  status: PayoutBatchStatus;
  total: Money;
  teacherCount: number;
  lineCount: number;
  createdAt: string;
  completedAt: string | null;
}

export interface PayoutBatchLine {
  teacher: { slug: string; displayName: string };
  amount: Money;
  lessonCount: number;
}

export interface PayoutBatchDetail extends PayoutBatch {
  lines: PayoutBatchLine[];
}

export interface PayoutDashboard {
  owed: OwedPayoutRow[];
  totals: PayoutTotals;
  batches: PayoutBatch[];
  batchesTotal: number;
}

const cur = (c: string) => c as Money["currency"];

const toPayoutBatch = (b: WirePayoutBatch): PayoutBatch => ({
  id: b.id,
  createdBy: { id: b.created_by.id, displayName: b.created_by.display_name },
  status: b.status,
  total: { amountMinor: b.total_minor, currency: cur(b.currency) },
  teacherCount: b.teacher_count,
  lineCount: b.line_count,
  createdAt: b.created_at,
  completedAt: b.completed_at ?? null,
});

const toPayoutBatchDetail = (b: WirePayoutBatchDetail): PayoutBatchDetail => ({
  ...toPayoutBatch(b),
  lines: b.lines.map((l) => ({
    teacher: { slug: l.teacher.slug, displayName: l.teacher.display_name },
    amount: { amountMinor: l.amount_minor, currency: cur(l.currency) },
    lessonCount: l.lesson_count,
  })),
});

export async function getPayoutDashboard(page = 1): Promise<PayoutDashboard> {
  const w = await call<WirePayoutDashboard>(
    `/v1/admin/payouts?page=${page}`,
    {},
    "Could not load the payout dashboard.",
  );
  return {
    owed: w.owed.map((o) => ({
      teacher: { slug: o.teacher.slug, displayName: o.teacher.display_name },
      available: { amountMinor: o.available_minor, currency: cur(o.currency) },
      oldestAvailableAt: o.oldest_available_at,
    })),
    totals: {
      available: {
        amountMinor: w.totals.available_total_minor,
        currency: cur(w.totals.currency),
      },
      held: {
        amountMinor: w.totals.held_total_minor,
        currency: cur(w.totals.currency),
      },
      paid: {
        amountMinor: w.totals.paid_total_minor,
        currency: cur(w.totals.currency),
      },
    },
    batches: w.batches.map(toPayoutBatch),
    batchesTotal: w.batches_total,
  };
}

export async function getPayoutBatch(id: string): Promise<PayoutBatchDetail> {
  const w = await call<WirePayoutBatchDetail>(
    `/v1/admin/payouts/batches/${encodeURIComponent(id)}`,
    {},
    "Could not load that payout batch.",
  );
  return toPayoutBatchDetail(w);
}

/** Settle every cleared earning. 409 `nothing_to_pay` when there is nothing. */
export async function runPayout(): Promise<PayoutBatchDetail> {
  const w = await call<WirePayoutBatchDetail>(
    "/v1/admin/payouts/run",
    { method: "POST", body: "{}" },
    "Could not run the payout.",
  );
  return toPayoutBatchDetail(w);
}
