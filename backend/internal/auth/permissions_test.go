package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type fakePermissions struct{ perms []string }

func (f fakePermissions) PermissionsFor(context.Context, uuid.UUID) ([]string, error) {
	return f.perms, nil
}

func TestMe_CarriesPermissions(t *testing.T) {
	repo := newFakeRepo()
	r, svc, _ := newTestHandler(repo)
	svc.SetPermissionsPort(fakePermissions{perms: []string{"metrics.view", "users.view"}})

	reg := decodeAuth(t, do(r, http.MethodPost, "/v1/auth/register",
		`{"email":"admin@x.com","password":"password123","display_name":"Admin"}`, nil))
	if len(reg.User.Permissions) != 2 {
		t.Fatalf("register response permissions = %v, want 2", reg.User.Permissions)
	}

	w := do(r, http.MethodGet, "/v1/auth/me", "", map[string]string{"Authorization": "Bearer " + reg.AccessToken})
	var me userDTO
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(me.Permissions) != 2 || me.Permissions[0] != "metrics.view" {
		t.Fatalf("me permissions = %v", me.Permissions)
	}
}

func TestMe_NilPermissionsPort_EmptyArray(t *testing.T) {
	repo := newFakeRepo()
	r, _, _ := newTestHandler(repo) // no SetPermissionsPort

	reg := decodeAuth(t, do(r, http.MethodPost, "/v1/auth/register",
		`{"email":"u@x.com","password":"password123","display_name":"U"}`, nil))

	w := do(r, http.MethodGet, "/v1/auth/me", "", map[string]string{"Authorization": "Bearer " + reg.AccessToken})
	// permissions must serialize as [] not null
	if !contains(w.Body.String(), `"permissions":[]`) {
		t.Fatalf("expected empty permissions array, got %s", w.Body.String())
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
