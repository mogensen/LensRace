// Package db manages the SQLite connection and schema migrations.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens (and creates, if missing) the SQLite database at path with
// foreign key enforcement turned on.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s?_pragma=foreign_keys(1)", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return conn, nil
}

// Migrate applies any embedded migration files that haven't already been
// recorded in schema_migrations, in filename order.
func Migrate(conn *sql.DB) error {
	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
	)`); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	names, err := migrationNames()
	if err != nil {
		return err
	}

	for _, name := range names {
		applied, err := isApplied(conn, name)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			continue
		}
		if err := applyMigration(conn, name); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}

	return nil
}

func migrationNames() ([]string, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func isApplied(conn *sql.DB, name string) (bool, error) {
	var count int
	err := conn.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, name).Scan(&count)
	return count > 0, err
}

func applyMigration(conn *sql.DB, name string) error {
	contents, err := migrationsFS.ReadFile("migrations/" + name)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, stmt := range splitStatements(string(contents)) {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("exec statement: %w", err)
		}
	}

	if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, name); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit()
}

// splitStatements does a naive split of a SQL script into individual
// statements on ";". It does not need to be smarter than that as long as
// migration files stay free of semicolons inside string literals/triggers.
func splitStatements(script string) []string {
	parts := strings.Split(script, ";")
	stmts := make([]string, 0, len(parts))
	for _, part := range parts {
		if stmt := strings.TrimSpace(part); stmt != "" {
			stmts = append(stmts, stmt)
		}
	}
	return stmts
}
