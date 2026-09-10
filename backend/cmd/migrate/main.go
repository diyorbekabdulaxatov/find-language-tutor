// Command migrate applies or rolls back database migrations using golang-migrate
// as a library (so we don't depend on the migrate CLI or its build tags).
//
//	go run ./cmd/migrate up          # apply all pending
//	go run ./cmd/migrate down 1      # roll back one
//	go run ./cmd/migrate version     # print current version
//	go run ./cmd/migrate force 3     # set version, clear dirty flag
package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/diyorbekabdulaxatov/find-language-tutor/backend/internal/config"
)

func main() {
	log.SetFlags(0)

	args := os.Args[1:]
	if len(args) == 0 {
		log.Fatal("usage: migrate <up|down|version|force> [n]")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	m, err := migrate.New("file://migrations", pgxURL(cfg.DatabaseURL))
	if err != nil {
		log.Fatalf("open migrate: %v", err)
	}
	defer m.Close()

	switch args[0] {
	case "up":
		err = m.Up()
	case "down":
		if len(args) > 1 {
			n, convErr := strconv.Atoi(args[1])
			if convErr != nil {
				log.Fatalf("down: %v", convErr)
			}
			err = m.Steps(-n)
		} else {
			err = m.Down()
		}
	case "version":
		v, dirty, vErr := m.Version()
		if errors.Is(vErr, migrate.ErrNilVersion) {
			fmt.Println("version=none (no migrations applied)")
			return
		}
		if vErr != nil {
			log.Fatal(vErr)
		}
		fmt.Printf("version=%d dirty=%t\n", v, dirty)
		return
	case "force":
		if len(args) < 2 {
			log.Fatal("force requires a version number")
		}
		n, convErr := strconv.Atoi(args[1])
		if convErr != nil {
			log.Fatalf("force: %v", convErr)
		}
		err = m.Force(n)
	default:
		log.Fatalf("unknown command %q", args[0])
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("%s: %v", args[0], err)
	}
	log.Printf("%s: ok", args[0])
}

// pgxURL rewrites a postgres:// connection string to the pgx5:// scheme that the
// golang-migrate pgx/v5 driver registers.
func pgxURL(databaseURL string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(databaseURL, prefix) {
			return "pgx5://" + strings.TrimPrefix(databaseURL, prefix)
		}
	}
	return databaseURL
}
