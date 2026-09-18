package files

import (
	"context"
	"io"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/teachers"
)

// TeacherGateway adapts *Service to teachers.FileReader: a profile's photo /
// intro video must be a file the owning account uploaded, and an approved
// profile's media is served publicly. teachers defines the port, this adapter
// implements it, cmd/api injects it with teachers.Service.SetFileReader.
// files never imports internal/teachers elsewhere.
type TeacherGateway struct{ svc *Service }

// NewTeacherGateway wraps the files service as a teachers.FileReader.
func NewTeacherGateway(svc *Service) *TeacherGateway { return &TeacherGateway{svc: svc} }

var _ teachers.FileReader = (*TeacherGateway)(nil)

func (g *TeacherGateway) FileOwnedBy(ctx context.Context, fileAssetID, callerID uuid.UUID) (bool, string, error) {
	return g.svc.FileOwnedBy(ctx, fileAssetID, callerID)
}

func (g *TeacherGateway) PublicAsset(ctx context.Context, fileAssetID uuid.UUID) (redirectURL string, body io.ReadCloser, contentType string, err error) {
	redirectURL, body, a, err := g.svc.ServePublic(ctx, fileAssetID)
	if err != nil {
		return "", nil, "", err
	}
	return redirectURL, body, a.ContentType, nil
}
