/**
 * Admin data-access (browser). Every call is behind `RequireAdmin` on the
 * backend.
 *
 * NOTE: the `/v1/admin/*` endpoints are being added (admin phases A/B). Until
 * they land in openapi.yaml + `npm run gen:api` this uses `authedFetch` with
 * hand-written wire types; swap to the typed `browserApi` once the schema
 * regenerates.
 */

import { authedFetch } from "@/features/auth/browser-client";
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
