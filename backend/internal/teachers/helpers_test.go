package teachers

import (
	"io"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/auth"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testTokenManager() *auth.TokenManager {
	return auth.NewTokenManager("teachers-test-secret", time.Minute)
}

func bearerFor(tm *auth.TokenManager, id uuid.UUID) string {
	tok, err := tm.IssueAccess(auth.User{ID: id, Email: "demo@example.com", DisplayName: "Demo"}, time.Now())
	if err != nil {
		panic(err)
	}
	return "Bearer " + tok
}
