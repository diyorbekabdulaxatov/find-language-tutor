/**
 * RBAC permission keys — mirror `backend/internal/rbac/permissions.go`. The
 * backend is the authority (every /v1/admin call is checked server-side); these
 * gate the UI so people only see what they can act on.
 */

export const PERMISSIONS = {
  metricsView: "metrics.view",
  usersView: "users.view",
  usersManageRoles: "users.manage_roles",
  teachersView: "teachers.view",
  teachersModerate: "teachers.moderate",
  teachersVerify: "teachers.verify",
  bookingsView: "bookings.view",
  bookingsForceCancel: "bookings.force_cancel",
  disputesResolve: "disputes.resolve",
  payoutsView: "payouts.view",
  payoutsRun: "payouts.run",
  reviewsModerate: "reviews.moderate",
  rolesManage: "roles.manage",
} as const;

export type Permission = (typeof PERMISSIONS)[keyof typeof PERMISSIONS];

/** Does this permission list grant `perm`? */
export function can(permissions: string[], perm: Permission): boolean {
  return permissions.includes(perm);
}

/** Any admin access at all — used to decide whether to show the /admin area. */
export function hasAnyAdminAccess(permissions: string[]): boolean {
  return permissions.length > 0;
}
