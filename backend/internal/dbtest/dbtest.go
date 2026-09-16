// Package dbtest gives integration tests a real, migrated Postgres.
//
// The unit suite (`make test`) never touches a database — fakes and httptest
// throughout. This package is for the handful of invariants only the
// database enforces: EXCLUDE constraints, CHECKs, unique-violation
// idempotency, jsonb round-trips, and that every migration rolls back.
//
// Tests using it are tagged `integration` and run with `make test-integration`.
// A database comes from TEST_DATABASE_URL when set (CI service, or a local
// `make db-up` instance — the test still resets the schema), otherwise from a
// throwaway postgres container via testcontainers, which needs Docker.
package dbtest

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/db"
)

// Image matches docker-compose.yml so the tests exercise the same server.
const Image = "postgres:16-alpine"

// One container serves every test in the package (each test still resets
// the schema through Pool). Terminate it from TestMain.
var shared struct {
	once      sync.Once
	url       string
	err       error
	terminate func()
}

// URL returns a connection string to a database the test may freely mutate,
// starting a container when TEST_DATABASE_URL is unset. Skips the test when
// neither is possible.
func URL(t *testing.T) string {
	t.Helper()
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		// Pool wipes the schema. Refuse anything that doesn't announce itself
		// as a test database, so a stray dev URL can't lose someone's data.
		if cfg, err := pgx.ParseConfig(u); err != nil || !strings.Contains(strings.ToLower(cfg.Database), "test") {
			t.Fatalf("TEST_DATABASE_URL must name a database containing \"test\" (got %q) — this suite drops the schema", u)
		}
		return u
	}
	shared.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		c, err := tcpostgres.Run(ctx, Image,
			tcpostgres.WithDatabase("findtutor_test"),
			tcpostgres.WithUsername("findtutor"),
			tcpostgres.WithPassword("findtutor"),
			tcpostgres.BasicWaitStrategies(),
		)
		if err != nil {
			shared.err = err
			return
		}
		shared.terminate = func() { _ = testcontainers.TerminateContainer(c) }
		shared.url, shared.err = c.ConnectionString(ctx, "sslmode=disable")
	})
	if shared.err != nil {
		t.Skipf("no TEST_DATABASE_URL and could not start a postgres container (is Docker running?): %v", shared.err)
	}
	return shared.url
}

// Terminate stops the shared container, if one was started. Call it from
// TestMain after m.Run().
func Terminate() {
	if shared.terminate != nil {
		shared.terminate()
	}
}

// Migrator opens golang-migrate against the repo's migrations directory.
func Migrator(t *testing.T, databaseURL string) *migrate.Migrate {
	t.Helper()
	m, err := migrate.New("file://"+migrationsDir(t), pgxURL(databaseURL))
	if err != nil {
		t.Fatalf("open migrate: %v", err)
	}
	t.Cleanup(func() { m.Close() })
	return m
}

// Pool returns a pool on a database migrated to the latest version, with
// every table emptied first so tests never see each other's rows.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	u := URL(t)
	ResetSchema(t, u)
	if err := Migrator(t, u).Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, u)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// ResetSchema wipes everything — tables, enum types, the migrations table —
// so a test starts from nothing even on a shared database. (golang-migrate's
// Drop only removes tables, which leaves the enum types behind.)
func ResetSchema(t *testing.T, databaseURL string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect for reset: %v", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate migrations directory")
	}
	// internal/dbtest/dbtest.go -> backend/migrations
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

// pgxURL rewrites postgres:// to the pgx5:// scheme golang-migrate's driver
// registers (same as cmd/migrate).
func pgxURL(databaseURL string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(databaseURL, prefix) {
			return "pgx5://" + strings.TrimPrefix(databaseURL, prefix)
		}
	}
	return databaseURL
}
