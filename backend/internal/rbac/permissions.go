package rbac

// Permission is a single capability key. The catalog below is the source of
// truth: role_permissions rows are validated against it on write, the
// GET /v1/admin/permissions endpoint serves it, and the `superadmin` system
// role always holds exactly AllPermissions (recomputed on seed).
type Permission string

const (
	// Phase A/B — wired to endpoints.
	PermMetricsView      Permission = "metrics.view"       // GET /v1/admin/metrics
	PermUsersView        Permission = "users.view"         // GET /v1/admin/users[/{id}]
	PermUsersManageRoles Permission = "users.manage_roles" // assign / unassign a user's roles
	PermTeachersView     Permission = "teachers.view"      // GET /v1/admin/teachers[/{slug}]
	PermTeachersModerate Permission = "teachers.moderate"  // approve / reject / suspend
	PermTeachersVerify   Permission = "teachers.verify"    // toggle the verified badge
	PermRolesManage      Permission = "roles.manage"       // role CRUD + the permission catalog

	// Phases C/D/E — defined now, no endpoint yet.
	PermBookingsView        Permission = "bookings.view"
	PermBookingsForceCancel Permission = "bookings.force_cancel"
	PermDisputesResolve     Permission = "disputes.resolve"
	PermPayoutsView         Permission = "payouts.view"
	PermPayoutsRun          Permission = "payouts.run"
	PermReviewsModerate     Permission = "reviews.moderate"
)

// PermissionInfo is one catalog entry for the GET /v1/admin/permissions UI.
type PermissionInfo struct {
	Key         Permission
	Description string
}

// Catalog is the full, ordered permission catalog.
var Catalog = []PermissionInfo{
	{PermMetricsView, "View the admin dashboard metrics."},
	{PermUsersView, "View the user directory and user detail pages."},
	{PermUsersManageRoles, "Assign and unassign roles on a user."},
	{PermTeachersView, "View teacher profiles in the admin moderation queue."},
	{PermTeachersModerate, "Approve, reject, and suspend teacher profiles."},
	{PermTeachersVerify, "Grant or remove a teacher's verified badge."},
	{PermRolesManage, "Create, edit, and delete roles and their permissions."},
	{PermBookingsView, "View any booking (not yet wired to an endpoint)."},
	{PermBookingsForceCancel, "Force-cancel a booking (not yet wired to an endpoint)."},
	{PermDisputesResolve, "Resolve payment disputes (not yet wired to an endpoint)."},
	{PermPayoutsView, "View the teacher payout ledger (not yet wired to an endpoint)."},
	{PermPayoutsRun, "Run a payout batch (not yet wired to an endpoint)."},
	{PermReviewsModerate, "Hide or remove reviews (not yet wired to an endpoint)."},
}

// AllPermissions is every permission key in catalog order.
var AllPermissions = func() []Permission {
	out := make([]Permission, len(Catalog))
	for i, p := range Catalog {
		out[i] = p.Key
	}
	return out
}()

var catalogSet = func() map[Permission]struct{} {
	m := make(map[Permission]struct{}, len(Catalog))
	for _, p := range Catalog {
		m[p.Key] = struct{}{}
	}
	return m
}()

// ValidPermission reports whether key is in the catalog.
func ValidPermission(key string) bool {
	_, ok := catalogSet[Permission(key)]
	return ok
}
