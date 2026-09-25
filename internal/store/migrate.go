package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"sort"

	"github.com/gabe-santos/yogurt/internal/version"
)

//go:embed migrations/*.sql
var migrations embed.FS

// migrate applies every embedded migration the database has not seen yet, in
// filename order, each in its own transaction, recording which Yogurt applied
// it. It refuses a database carrying a migration this binary does not embed:
// a newer Yogurt applied it, and this one does not know the schema it left.
func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name TEXT PRIMARY KEY,
		applied_at INTEGER NOT NULL
	) STRICT`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// applied_by came after the table itself, so every database gains it here,
	// new or not; rows from before versioning keep it empty.
	var versioned bool
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) > 0 FROM pragma_table_info('schema_migrations') WHERE name = 'applied_by'`,
	).Scan(&versioned); err != nil {
		return fmt.Errorf("inspect schema_migrations: %w", err)
	}
	if !versioned {
		if _, err := db.ExecContext(ctx,
			`ALTER TABLE schema_migrations ADD COLUMN applied_by TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("add schema_migrations.applied_by: %w", err)
		}
	}

	applied, err := appliedMigrations(ctx, db)
	if err != nil {
		return err
	}

	names, err := migrationNames()
	if err != nil {
		return err
	}

	newest := ""
	for name := range applied {
		if _, known := slices.BinarySearch(names, name); !known && name > newest {
			newest = name
		}
	}
	if newest != "" {
		return fmt.Errorf("database was migrated by Yogurt %s, newer than this Yogurt %s", applied[newest], version.Version)
	}

	for _, name := range names {
		if _, done := applied[name]; done {
			continue
		}
		statements, err := fs.ReadFile(migrations, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := applyMigration(ctx, db, name, string(statements)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, name, statements string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, statements); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (name, applied_at, applied_by) VALUES (?, unixepoch(), ?)`,
		name, version.Version); err != nil {
		return err
	}
	return tx.Commit()
}

// appliedMigrations maps each migration the database has applied to the
// version of Yogurt that applied it.
func appliedMigrations(ctx context.Context, db *sql.DB) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT name, applied_by FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]string)
	for rows.Next() {
		var name, appliedBy string
		if err := rows.Scan(&name, &appliedBy); err != nil {
			return nil, err
		}
		applied[name] = appliedBy
	}
	return applied, rows.Err()
}

func migrationNames() ([]string, error) {
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}
