package files

import (
	"context"

	"github.com/google/uuid"
)

// AssigneeChecker widens Download's access rule beyond "owner only": a student
// assigned the file (through a resource attached to one of their lessons) may
// also read it. Optional port (SetAssigneeChecker); a nil checker preserves
// today's owner-only behavior. internal/resources provides the adapter
// (resources.NewFileGateway); this package never imports internal/resources.
type AssigneeChecker interface {
	CanAccess(ctx context.Context, fileAssetID, requesterID uuid.UUID) (bool, error)
}
