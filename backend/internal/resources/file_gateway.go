package resources

import (
	"context"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/files"
)

// FileGateway adapts *Service to files.AssigneeChecker: a student assigned a
// resource that carries a file may download it, not just the file's owner.
// files defines the port, this adapter implements it, cmd/api injects it with
// files.Service.SetAssigneeChecker. files never imports this package.
type FileGateway struct{ svc *Service }

// NewFileGateway wraps the resources service as a files.AssigneeChecker.
func NewFileGateway(svc *Service) *FileGateway { return &FileGateway{svc: svc} }

var _ files.AssigneeChecker = (*FileGateway)(nil)

func (g *FileGateway) CanAccess(ctx context.Context, fileAssetID, requesterID uuid.UUID) (bool, error) {
	return g.svc.CanAccessFile(ctx, fileAssetID, requesterID)
}
