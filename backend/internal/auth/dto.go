package auth

// Wire DTOs — the source of truth for the JSON shape, in sync with openapi.yaml
// (snake_case). The refresh token is never in a body; it travels as the
// ftr_session cookie.

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// updateMeRequest is the body of PATCH /v1/auth/me. Only display_name is
// editable; a nil pointer means "field omitted" (no change). Email is read-only
// here and any email key in the body is ignored.
type updateMeRequest struct {
	DisplayName *string `json:"display_name"`
}

type userDTO struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	Permissions []string `json:"permissions"` // effective RBAC permission keys, sorted; the frontend gates /admin on these
}

// authResponse is returned by register / login / refresh. The access token is
// meant to be held in memory by the client and sent as `Authorization: Bearer`.
type authResponse struct {
	AccessToken string  `json:"access_token"`
	ExpiresIn   int     `json:"expires_in"` // access-token lifetime, seconds
	User        userDTO `json:"user"`
}

func toUserDTO(u User, permissions []string) userDTO {
	if permissions == nil {
		permissions = []string{}
	}
	return userDTO{ID: u.ID.String(), Email: u.Email, DisplayName: u.DisplayName, Permissions: permissions}
}

func toAuthResponse(r AuthResult) authResponse {
	return authResponse{
		AccessToken: r.AccessToken,
		ExpiresIn:   int(r.AccessTTL.Seconds()),
		User:        toUserDTO(r.User, r.Permissions),
	}
}
