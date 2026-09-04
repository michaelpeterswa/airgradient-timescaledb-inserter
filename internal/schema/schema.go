// Package schema applies the embedded database migrations. It exists so the
// service can run `migrate up` as an init container against the cluster's
// CloudNativePG TimescaleDB before the inserter starts, with no separate
// migrations image to keep in step with the code.
package schema

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

//go:embed migrations/*.sql
var migrations embed.FS

// MigrationsTable is where golang-migrate records the applied version. It is
// namespaced to this service so that a database shared with other inserters,
// or with the central timescale-migrations repository, keeps one history per
// owner instead of one contested schema_migrations table.
const MigrationsTable = "airgradient_schema_migrations"

// Migrator wraps a golang-migrate instance over the embedded migrations.
type Migrator struct {
	m  *migrate.Migrate
	db *sql.DB
}

// New connects to the database and prepares the embedded migrations. The
// connection string is the same postgres:// URL the inserter uses.
func New(connString string) (*Migrator, error) {
	src, err := iofs.New(migrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("load embedded migrations: %w", err)
	}

	db, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{
		MigrationsTable: MigrationsTable,
	})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create migrator: %w", err)
	}
	return &Migrator{m: m, db: db}, nil
}

// Close releases the source and database handles.
func (mg *Migrator) Close() error {
	srcErr, dbErr := mg.m.Close()
	return errors.Join(srcErr, dbErr)
}

// Up applies every pending migration. It returns false when there was nothing
// to do, which is the usual case for an init container on a healthy cluster.
func (mg *Migrator) Up() (applied bool, err error) {
	err = mg.m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("migrate up: %w", err)
	}
	return true, nil
}

// Down rolls back the most recent migration. It is deliberately one step at
// a time, because the down migrations drop hypertables and their data.
func (mg *Migrator) Down() error {
	err := mg.m.Steps(-1)
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

// Version reports the current schema version and whether a failed migration
// left it dirty. A dirty database refuses further migrations until it is
// repaired with Force.
func (mg *Migrator) Version() (version uint, dirty bool, err error) {
	version, dirty, err = mg.m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read version: %w", err)
	}
	return version, dirty, nil
}

// Force marks the database as being at a version and clean, without running
// anything. Use it to recover after a migration failed part way and the
// operator has fixed the database by hand.
func (mg *Migrator) Force(version int) error {
	if err := mg.m.Force(version); err != nil {
		return fmt.Errorf("force version %d: %w", version, err)
	}
	return nil
}
