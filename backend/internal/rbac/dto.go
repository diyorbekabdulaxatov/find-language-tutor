package rbac

// Wire DTOs — the source of truth for the JSON shape, in sync with openapi.yaml
// (snake_case).

type permissionInfoDTO struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

func toCatalogDTO(cat []PermissionInfo) []permissionInfoDTO {
	out := make([]permissionInfoDTO, len(cat))
	for i, p := range cat {
		out[i] = permissionInfoDTO{Key: string(p.Key), Description: p.Description}
	}
	return out
}

type roleDTO struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IsSystem    bool     `json:"is_system"`
	Permissions []string `json:"permissions"`
	UserCount   int64    `json:"user_count"`
}

func toRoleDTO(r Role) roleDTO {
	perms := make([]string, len(r.Permissions))
	for i, p := range r.Permissions {
		perms[i] = string(p)
	}
	return roleDTO{
		ID:          r.ID.String(),
		Name:        r.Name,
		Description: r.Description,
		IsSystem:    r.IsSystem,
		Permissions: perms,
		UserCount:   r.UserCount,
	}
}

func toRoleListDTO(rs []Role) []roleDTO {
	out := make([]roleDTO, len(rs))
	for i, r := range rs {
		out[i] = toRoleDTO(r)
	}
	return out
}

type createRoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type updateRoleRequest struct {
	Description *string   `json:"description"`
	Permissions *[]string `json:"permissions"`
}

type assignRoleRequest struct {
	RoleID string `json:"role_id"`
}
