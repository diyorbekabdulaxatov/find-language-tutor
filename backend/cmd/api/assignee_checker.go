package main

import (
	"context"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/files"
)

// multiAssigneeChecker composes several files.AssigneeChecker widenings into
// one: files.Service has only a single AssigneeChecker slot (SetAssigneeChecker),
// so phase C2 needing a second, independent widening (a course-enrolled
// student's video access, alongside phase A2's lesson-assigned-resource
// access) is wired here rather than by changing internal/files' one-checker
// design. Pure wiring glue — deliberately kept in cmd/api, not in
// internal/files.
type multiAssigneeChecker []files.AssigneeChecker

var _ files.AssigneeChecker = multiAssigneeChecker(nil)

func (m multiAssigneeChecker) CanAccess(ctx context.Context, fileAssetID, requesterID uuid.UUID) (bool, error) {
	for _, checker := range m {
		ok, err := checker.CanAccess(ctx, fileAssetID, requesterID)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}
