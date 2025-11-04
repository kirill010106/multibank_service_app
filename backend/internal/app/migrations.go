package app

import (
	"embed"

	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// getEmbeddedMigrations returns a source.Driver for embedded migrations
func getEmbeddedMigrations() (source.Driver, error) {
	return iofs.New(migrationsFS, "migrations")
}
