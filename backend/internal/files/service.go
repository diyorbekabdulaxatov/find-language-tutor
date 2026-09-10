package files

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Repository persists the file_assets rows.
type Repository interface {
	Create(ctx context.Context, a Asset) (Asset, error)
	ByID(ctx context.Context, id uuid.UUID) (Asset, error) // ErrAssetNotFound when absent
}

// Service holds the upload rules. Handlers call it; it never sees a *gin.Context.
type Service struct {
	repo   Repository
	blob   Blob
	logger *slog.Logger
}

func NewService(repo Repository, blob Blob, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{repo: repo, blob: blob, logger: logger}
}

// Upload validates the file, writes the bytes to the blob store, and records
// the asset. size is the declared size; the reader is also length-limited as a
// backstop.
func (s *Service) Upload(ctx context.Context, ownerID uuid.UUID, filename, contentType string, size int64, r io.Reader) (Asset, error) {
	contentType = normalizeContentType(contentType)
	ext, ok := allowedContentTypes[contentType]
	if !ok {
		return Asset{}, ErrUnsupportedType
	}
	if size <= 0 {
		return Asset{}, ErrEmptyUpload
	}
	if size > MaxUploadBytes {
		return Asset{}, ErrTooLarge
	}

	key := "resources/" + ownerID.String() + "/" + uuid.NewString() + ext
	limited := io.LimitReader(r, MaxUploadBytes+1)
	if err := s.blob.Put(ctx, key, contentType, limited); err != nil {
		return Asset{}, err
	}

	asset, err := s.repo.Create(ctx, Asset{
		OwnerID:     ownerID,
		Provider:    s.blob.Provider(),
		ObjectKey:   key,
		Filename:    sanitizeFilename(filename),
		ContentType: contentType,
		Bytes:       size,
	})
	if err != nil {
		// best-effort cleanup of the orphaned blob
		_ = s.blob.Delete(ctx, key)
		return Asset{}, err
	}
	return asset, nil
}

// Download resolves an asset for a caller. Returns either a redirect URL (R2)
// or an open reader (disk). The caller must Close the reader when non-nil.
//
// Access rule (phase A1): owner only. Phase A2 widens this to a student the
// resource has been assigned to.
func (s *Service) Download(ctx context.Context, id, requesterID uuid.UUID) (redirectURL string, body io.ReadCloser, a Asset, err error) {
	a, err = s.repo.ByID(ctx, id)
	if err != nil {
		return "", nil, Asset{}, err
	}
	if a.OwnerID != requesterID {
		return "", nil, Asset{}, ErrForbidden
	}

	if url, ok, e := s.blob.PresignedGetURL(ctx, a.ObjectKey, 5*time.Minute); e == nil && ok {
		return url, nil, a, nil
	}
	rc, e := s.blob.Open(ctx, a.ObjectKey)
	if e != nil {
		return "", nil, Asset{}, e
	}
	return "", rc, a, nil
}

func normalizeContentType(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.ToLower(strings.TrimSpace(ct))
}

// sanitizeFilename keeps a human label without path separators or control chars.
func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r < 0x20 {
			return '-'
		}
		return r
	}, name)
	if len(name) > 200 {
		name = name[:200]
	}
	if name == "" {
		return "file"
	}
	return name
}
