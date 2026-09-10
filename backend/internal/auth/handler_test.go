package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

const testCookieName = "ftr_session"

func newTestHandler(repo Repository) (*gin.Engine, *Service, *TokenManager) {
	gin.SetMode(gin.TestMode)
	svc, tm := newTestService(repo)
	h := NewHandler(svc, tm, CookieConfig{
		Name:   testCookieName,
		Path:   "/v1/auth",
		Secure: false,
		MaxAge: 24 * time.Hour,
	}, discardLogger())
	r := gin.New()
	RegisterRoutes(r.Group("/v1/auth"), h)
	return r, svc, tm
}

func do(r http.Handler, method, path, body string, headers map[string]string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	var rdr *strings.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	} else {
		rdr = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func cookieNamed(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range (&http.Response{Header: w.Header()}).Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func decodeAuth(t *testing.T, w *httptest.ResponseRecorder) authResponse {
	t.Helper()
	var body authResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (body %s)", err, w.Body.String())
	}
	return body
}

func TestHandler_FullFlow(t *testing.T) {
	r, _, _ := newTestHandler(newFakeRepo())

	// register
	w := do(r, http.MethodPost, "/v1/auth/register", `{"email":"flow@example.com","password":"password123","display_name":"Flow"}`, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body %s", w.Code, w.Body.String())
	}
	reg := decodeAuth(t, w)
	if reg.AccessToken == "" || reg.User.Email != "flow@example.com" || reg.ExpiresIn <= 0 {
		t.Fatalf("register body wrong: %+v", reg)
	}
	regCookie := cookieNamed(w, testCookieName)
	if regCookie == nil || !regCookie.HttpOnly || regCookie.Value == "" {
		t.Fatalf("refresh cookie missing/not HttpOnly: %+v", regCookie)
	}
	if regCookie.SameSite != http.SameSiteLaxMode || regCookie.Path != "/v1/auth" {
		t.Errorf("cookie attrs: samesite=%v path=%q", regCookie.SameSite, regCookie.Path)
	}

	// /me with the access token
	w = do(r, http.MethodGet, "/v1/auth/me", "", map[string]string{"Authorization": "Bearer " + reg.AccessToken})
	if w.Code != http.StatusOK {
		t.Fatalf("/me status = %d", w.Code)
	}
	var me userDTO
	_ = json.Unmarshal(w.Body.Bytes(), &me)
	if me.Email != "flow@example.com" {
		t.Errorf("/me body = %+v", me)
	}

	// refresh rotates
	w = do(r, http.MethodPost, "/v1/auth/refresh", "", nil, regCookie)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh status = %d, body %s", w.Code, w.Body.String())
	}
	ref := decodeAuth(t, w)
	newCookie := cookieNamed(w, testCookieName)
	if newCookie == nil || newCookie.Value == regCookie.Value {
		t.Fatalf("refresh did not set a new cookie value")
	}

	// new access token still works on /me
	w = do(r, http.MethodGet, "/v1/auth/me", "", map[string]string{"Authorization": "Bearer " + ref.AccessToken})
	if w.Code != http.StatusOK {
		t.Fatalf("/me after refresh = %d", w.Code)
	}

	// the OLD refresh cookie is now rejected (and, being reuse, kills the chain)
	w = do(r, http.MethodPost, "/v1/auth/refresh", "", nil, regCookie)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("reused refresh cookie status = %d, want 401", w.Code)
	}

	// logout (chain already dead, but must still 204 + clear cookie)
	w = do(r, http.MethodPost, "/v1/auth/logout", "", nil, newCookie)
	if w.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", w.Code)
	}
	cleared := cookieNamed(w, testCookieName)
	if cleared == nil || cleared.MaxAge >= 0 && cleared.Value != "" {
		t.Errorf("logout did not clear the cookie: %+v", cleared)
	}

	// refresh after logout fails
	w = do(r, http.MethodPost, "/v1/auth/refresh", "", nil, newCookie)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout = %d, want 401", w.Code)
	}
}

func TestHandler_DuplicateEmail_409(t *testing.T) {
	r, _, _ := newTestHandler(newFakeRepo())
	body := `{"email":"dup@example.com","password":"password123","display_name":"Dup"}`

	if w := do(r, http.MethodPost, "/v1/auth/register", body, nil); w.Code != http.StatusCreated {
		t.Fatalf("first register = %d", w.Code)
	}
	w := do(r, http.MethodPost, "/v1/auth/register", body, nil)
	if w.Code != http.StatusConflict {
		t.Fatalf("duplicate register = %d, want 409", w.Code)
	}
	var e struct {
		Error struct{ Code string } `json:"error"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &e)
	if e.Error.Code != "email_taken" {
		t.Errorf("error code = %q", e.Error.Code)
	}
}

func TestHandler_Register_ValidationErrors(t *testing.T) {
	r, _, _ := newTestHandler(newFakeRepo())
	for _, body := range []string{
		`{"email":"bad","password":"password123","display_name":"X"}`,
		`{"email":"a@b.com","password":"short","display_name":"X"}`,
		`not json`,
	} {
		w := do(r, http.MethodPost, "/v1/auth/register", body, nil)
		if w.Code != http.StatusBadRequest {
			t.Errorf("body %q -> status %d, want 400", body, w.Code)
		}
	}
}

func TestHandler_Login_BadCredentials_401(t *testing.T) {
	r, _, _ := newTestHandler(newFakeRepo())
	_ = do(r, http.MethodPost, "/v1/auth/register", `{"email":"u@example.com","password":"password123","display_name":"U"}`, nil)

	w := do(r, http.MethodPost, "/v1/auth/login", `{"email":"u@example.com","password":"nope"}`, nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHandler_UpdateMe(t *testing.T) {
	r, _, _ := newTestHandler(newFakeRepo())

	reg := decodeAuth(t, do(r, http.MethodPost, "/v1/auth/register",
		`{"email":"patch@example.com","password":"password123","display_name":"Before"}`, nil))
	authHdr := map[string]string{"Authorization": "Bearer " + reg.AccessToken}

	// happy path
	w := do(r, http.MethodPatch, "/v1/auth/me", `{"display_name":"After"}`, authHdr)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	var me userDTO
	_ = json.Unmarshal(w.Body.Bytes(), &me)
	if me.DisplayName != "After" || me.Email != "patch@example.com" {
		t.Fatalf("body = %+v", me)
	}

	// email in the body is ignored
	w = do(r, http.MethodPatch, "/v1/auth/me", `{"email":"hacker@example.com"}`, authHdr)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	_ = json.Unmarshal(w.Body.Bytes(), &me)
	if me.Email != "patch@example.com" {
		t.Errorf("email changed: %+v", me)
	}

	// blank display name -> 400
	if w := do(r, http.MethodPatch, "/v1/auth/me", `{"display_name":"  "}`, authHdr); w.Code != http.StatusBadRequest {
		t.Errorf("blank name status = %d, want 400", w.Code)
	}

	// no token -> 401
	if w := do(r, http.MethodPatch, "/v1/auth/me", `{"display_name":"X"}`, nil); w.Code != http.StatusUnauthorized {
		t.Errorf("no token status = %d, want 401", w.Code)
	}
}

func TestHandler_Me_Unauthorized(t *testing.T) {
	r, _, tm := newTestHandler(newFakeRepo())

	// absent
	if w := do(r, http.MethodGet, "/v1/auth/me", "", nil); w.Code != http.StatusUnauthorized {
		t.Errorf("no header: %d", w.Code)
	}
	// malformed
	if w := do(r, http.MethodGet, "/v1/auth/me", "", map[string]string{"Authorization": "Bearer garbage"}); w.Code != http.StatusUnauthorized {
		t.Errorf("garbage: %d", w.Code)
	}
	// not a bearer
	if w := do(r, http.MethodGet, "/v1/auth/me", "", map[string]string{"Authorization": "Basic abc"}); w.Code != http.StatusUnauthorized {
		t.Errorf("basic: %d", w.Code)
	}
	// expired (valid signature, past exp)
	expired, _ := tm.IssueAccess(User{ID: testUser().ID, Email: "x@y.z"}, time.Now().Add(-time.Hour))
	if w := do(r, http.MethodGet, "/v1/auth/me", "", map[string]string{"Authorization": "Bearer " + expired}); w.Code != http.StatusUnauthorized {
		t.Errorf("expired: %d", w.Code)
	}
}
