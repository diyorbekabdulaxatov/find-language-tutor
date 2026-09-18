package teachers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
)

// fakeFiles is an in-memory FileReader: owned maps asset id -> (owner,
// content type); served records which asset PublicAsset was asked for.
type fakeFiles struct {
	owned map[uuid.UUID]struct {
		owner uuid.UUID
		ct    string
	}
	served []uuid.UUID
}

func (f *fakeFiles) FileOwnedBy(_ context.Context, id, caller uuid.UUID) (bool, string, error) {
	a, ok := f.owned[id]
	if !ok || a.owner != caller {
		return false, "", nil
	}
	return true, a.ct, nil
}

func (f *fakeFiles) PublicAsset(_ context.Context, id uuid.UUID) (string, io.ReadCloser, string, error) {
	f.served = append(f.served, id)
	return "", io.NopCloser(strings.NewReader("bytes")), f.owned[id].ct, nil
}

func mediaFixture(owner uuid.UUID) (*fakeRepo, *fakeFiles, uuid.UUID, uuid.UUID) {
	photo, video := uuid.New(), uuid.New()
	files := &fakeFiles{owned: map[uuid.UUID]struct {
		owner uuid.UUID
		ct    string
	}{
		photo: {owner, "image/jpeg"},
		video: {owner, "video/mp4"},
	}}
	repo := &fakeRepo{
		bySlug:      map[string]*Teacher{},
		ownerBySlug: map[string]uuid.UUID{},
	}
	return repo, files, photo, video
}

func TestService_Create_DerivesMediaURLsFromAssets(t *testing.T) {
	owner := uuid.New()
	repo, files, photo, video := mediaFixture(owner)
	svc := newVerifiedService(repo)
	svc.SetFileReader(files)

	in := validCreateInput()
	in.AvatarURL = "https://example.com/pasted.jpg" // an asset id wins over a pasted URL
	in.AvatarAssetID = &photo
	in.IntroVideoAssetID = &video
	if _, err := svc.Create(context.Background(), owner, in); err != nil {
		t.Fatalf("create: %v", err)
	}
	got := repo.createdInput
	if got.AvatarURL != "/v1/teachers/nodira-karimova/media/avatar" {
		t.Errorf("avatar url = %q", got.AvatarURL)
	}
	if got.IntroVideoURL != "/v1/teachers/nodira-karimova/media/intro-video" {
		t.Errorf("intro video url = %q", got.IntroVideoURL)
	}
	if got.AvatarAssetID == nil || *got.AvatarAssetID != photo || got.IntroVideoAssetID == nil || *got.IntroVideoAssetID != video {
		t.Errorf("asset ids not persisted: %+v", got)
	}
}

func TestService_Create_RejectsForeignOrWrongTypeMedia(t *testing.T) {
	owner := uuid.New()
	repo, files, photo, video := mediaFixture(owner)
	svc := newVerifiedService(repo)
	svc.SetFileReader(files)

	cases := map[string]func(*ProfileInput){
		"video as photo": func(in *ProfileInput) { in.AvatarAssetID = &video },
		"photo as video": func(in *ProfileInput) { in.IntroVideoAssetID = &photo },
		"someone else's": func(in *ProfileInput) { id := uuid.New(); in.AvatarAssetID = &id },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := validCreateInput()
			mutate(&in)
			_, err := svc.Create(context.Background(), owner, in)
			var ve ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want ValidationError", err)
			}
			if repo.createdInput != nil {
				t.Error("profile was created despite the bad asset")
			}
		})
	}
}

func TestService_Create_MediaFailsClosedWithoutFileReader(t *testing.T) {
	owner := uuid.New()
	repo, _, photo, _ := mediaFixture(owner)
	svc := newVerifiedService(repo) // no SetFileReader

	in := validCreateInput()
	in.AvatarAssetID = &photo
	var ve ValidationError
	if _, err := svc.Create(context.Background(), owner, in); !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

func TestService_Update_MediaPatchSemantics(t *testing.T) {
	owner := uuid.New()
	repo, files, photo, video := mediaFixture(owner)
	oldPhoto := uuid.New()
	files.owned[oldPhoto] = struct {
		owner uuid.UUID
		ct    string
	}{owner, "image/png"}
	repo.bySlug["nodira-karimova"] = &Teacher{
		ID: uuid.New(), Slug: "nodira-karimova", DisplayName: "Nodira Karimova", Headline: "h",
		Kind: KindProfessional, CountryCode: "UZ", CountryName: "Uzbekistan", City: "Tashkent",
		Timezone: "Asia/Tashkent", PricePerHour: Money{1, CurrencyUZS}, Status: StatusApproved,
		AvatarURL: "/v1/teachers/nodira-karimova/media/avatar", AvatarAssetID: &oldPhoto,
		IntroVideoURL: "https://example.com/pasted.mp4",
	}
	repo.ownerBySlug["nodira-karimova"] = owner
	svc := newVerifiedService(repo)
	svc.SetFileReader(files)
	ctx := context.Background()

	// Omitted: the current asset and its derived URL carry through.
	if _, err := svc.Update(ctx, "nodira-karimova", owner, ProfilePatch{}); err != nil {
		t.Fatal(err)
	}
	if f := repo.lastUpdate.Fields; f.AvatarAssetID == nil || *f.AvatarAssetID != oldPhoto || f.AvatarURL != "/v1/teachers/nodira-karimova/media/avatar" {
		t.Errorf("omitted asset changed: %+v", f)
	}

	// Replaced: new asset, same derived path; the pasted intro URL is
	// replaced by the media path once a video is uploaded.
	if _, err := svc.Update(ctx, "nodira-karimova", owner, ProfilePatch{
		AvatarAssetID:     OptionalUUID{Set: true, Value: &photo},
		IntroVideoAssetID: OptionalUUID{Set: true, Value: &video},
	}); err != nil {
		t.Fatal(err)
	}
	if f := repo.lastUpdate.Fields; *f.AvatarAssetID != photo || f.IntroVideoURL != "/v1/teachers/nodira-karimova/media/intro-video" {
		t.Errorf("replace wrong: %+v", f)
	}

	// Cleared (explicit null): asset gone AND the dead media path is blanked.
	if _, err := svc.Update(ctx, "nodira-karimova", owner, ProfilePatch{
		AvatarAssetID: OptionalUUID{Set: true, Value: nil},
	}); err != nil {
		t.Fatal(err)
	}
	if f := repo.lastUpdate.Fields; f.AvatarAssetID != nil || f.AvatarURL != "" {
		t.Errorf("clear wrong: asset=%v url=%q", f.AvatarAssetID, f.AvatarURL)
	}
}

func TestService_Media_OnlyApprovedProfilesServe(t *testing.T) {
	owner := uuid.New()
	repo, files, photo, _ := mediaFixture(owner)
	svc := NewService(repo)
	svc.SetFileReader(files)
	ctx := context.Background()

	teacher := &Teacher{Slug: "n", Status: StatusPending, AvatarAssetID: &photo}
	repo.bySlug["n"] = teacher

	for _, status := range []Status{StatusPending, StatusRejected, StatusSuspended} {
		teacher.Status = status
		if _, _, _, err := svc.Media(ctx, "n", MediaAvatar); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", status, err)
		}
	}
	teacher.Status = StatusApproved
	// empty slot
	if _, _, _, err := svc.Media(ctx, "n", MediaIntroVideo); !errors.Is(err, ErrNotFound) {
		t.Errorf("empty slot: err = %v, want ErrNotFound", err)
	}
	// unknown kind
	if _, _, _, err := svc.Media(ctx, "n", MediaKind("thumbnail")); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown kind: err = %v, want ErrNotFound", err)
	}
	_, body, ct, err := svc.Media(ctx, "n", MediaAvatar)
	if err != nil {
		t.Fatal(err)
	}
	body.Close()
	if ct != "image/jpeg" || len(files.served) != 1 || files.served[0] != photo {
		t.Errorf("served wrong: ct=%q served=%v", ct, files.served)
	}
	// unknown slug
	if _, _, _, err := svc.Media(ctx, "nobody", MediaAvatar); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown slug: err = %v", err)
	}
}

// seekableBody is what the disk store hands back: an *os.File-like reader
// http.ServeContent can Range over.
type seekableBody struct{ *strings.Reader }

func (seekableBody) Close() error { return nil }

func TestHandler_Media_ServesApprovedWithRange(t *testing.T) {
	owner := uuid.New()
	repo, files, photo, _ := mediaFixture(owner)
	repo.bySlug["n"] = &Teacher{Slug: "n", Status: StatusApproved, AvatarAssetID: &photo}
	seekFiles := &seekableFiles{fakeFiles: files, data: "0123456789"}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := NewService(repo)
	svc.SetFileReader(seekFiles)
	RegisterRoutes(r.Group("/v1/teachers"), NewHandler(svc, discardLogger()), auth.RequireAuth(testTokenManager()))

	// no auth header at all
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/teachers/n/media/avatar", nil))
	if w.Code != http.StatusOK || w.Body.String() != "0123456789" || w.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("full: status=%d ct=%q body=%q", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}

	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/teachers/n/media/avatar", nil)
	req.Header.Set("Range", "bytes=2-4")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusPartialContent || w.Body.String() != "234" {
		t.Errorf("range: status=%d body=%q", w.Code, w.Body.String())
	}

	for _, path := range []string{"/v1/teachers/n/media/intro-video", "/v1/teachers/n/media/thumbnail", "/v1/teachers/x/media/avatar"} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("%s: status=%d, want 404", path, w.Code)
		}
	}
}

type seekableFiles struct {
	*fakeFiles
	data string
}

func (f *seekableFiles) PublicAsset(ctx context.Context, id uuid.UUID) (string, io.ReadCloser, string, error) {
	_, _, ct, err := f.fakeFiles.PublicAsset(ctx, id)
	return "", seekableBody{strings.NewReader(f.data)}, ct, err
}

func TestHandler_Update_MediaNullVsOmitted(t *testing.T) {
	tm := testTokenManager()
	owner := uuid.New()
	repo, files, photo, _ := mediaFixture(owner)
	repo.bySlug["n"] = &Teacher{
		ID: uuid.New(), Slug: "n", DisplayName: "N", Headline: "h", Kind: KindProfessional,
		CountryCode: "UZ", CountryName: "Uzbekistan", City: "Tashkent", Timezone: "Asia/Tashkent",
		PricePerHour: Money{1, CurrencyUZS}, AvatarAssetID: &photo, AvatarURL: "/v1/teachers/n/media/avatar",
	}
	repo.ownerBySlug["n"] = owner

	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := newVerifiedService(repo)
	svc.SetFileReader(files)
	RegisterRoutes(r.Group("/v1/teachers"), NewHandler(svc, discardLogger()), auth.RequireAuth(tm))

	patch := func(body string) profileDTO {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, "/v1/teachers/n", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", bearerFor(tm, owner))
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
		}
		var out profileDTO
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}

	// omitted → unchanged (and the read DTO exposes the asset id)
	if got := patch(`{"headline":"h2"}`); got.AvatarAssetID == nil || *got.AvatarAssetID != photo.String() {
		t.Errorf("omitted: asset id = %v", got.AvatarAssetID)
	}
	// explicit null → cleared
	if got := patch(`{"avatar_asset_id":null}`); got.AvatarAssetID != nil || got.AvatarURL != "" {
		t.Errorf("null: asset=%v url=%q", got.AvatarAssetID, got.AvatarURL)
	}
	// bad uuid → 400, not a 500
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/v1/teachers/n", strings.NewReader(`{"avatar_asset_id":"nope"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", bearerFor(tm, owner))
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("bad uuid: status = %d", w.Code)
	}
}
