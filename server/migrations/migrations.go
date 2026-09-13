// Package migrations embeds Attic's SQL migrations and applies them.
//
// Migrations are forward-only and additive; they run automatically at startup
// so a fresh `docker compose up` yields a ready database with no extra step.
package migrations

import (
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // registers the pgx5:// migration driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed *.sql
var files embed.FS

// Up applies every pending migration against databaseURL.
func Up(databaseURL string, log *slog.Logger) error {
	source, err := iofs.New(files, ".")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, "pgx5://"+trimScheme(databaseURL))
	if err != nil {
		return fmt.Errorf("open migrator: %w", err)
	}
	defer m.Close()

	before, _, _ := m.Version()
	switch err := m.Up(); {
	case errors.Is(err, migrate.ErrNoChange):
		log.Info("database schema up to date", "version", before)
		return nil
	case err != nil:
		return fmt.Errorf("apply migrations: %w", err)
	}

	after, _, _ := m.Version()
	log.Info("database schema migrated", "from", before, "to", after)
	return nil
}

// trimScheme strips the postgres:// or postgresql:// prefix so the URL can be
// re-prefixed with the golang-migrate driver name.
func trimScheme(url string) string {
	for _, prefix := range []string{"postgres://", "postgresql://", "pgx5://", "pgx://"} {
		if len(url) >= len(prefix) && url[:len(prefix)] == prefix {
			return url[len(prefix):]
		}
	}
	return url
}
