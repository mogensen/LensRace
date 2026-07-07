package db

import (
	"database/sql"
	"testing"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	conn, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := Migrate(conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return conn
}

func TestMigrateAppliesAllMigrations(t *testing.T) {
	conn := openTestDB(t)

	names, err := migrationNames()
	if err != nil {
		t.Fatalf("migrationNames: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("expected at least one embedded migration")
	}

	var count int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if count != len(names) {
		t.Fatalf("schema_migrations count = %d, want %d", count, len(names))
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	conn := openTestDB(t)

	if err := Migrate(conn); err != nil {
		t.Fatalf("second Migrate call: %v", err)
	}
}

func TestSplitStatements(t *testing.T) {
	script := "CREATE TABLE a (id TEXT);\n\nINSERT INTO a VALUES ('x');\n"
	stmts := splitStatements(script)
	if len(stmts) != 2 {
		t.Fatalf("len(stmts) = %d, want 2: %v", len(stmts), stmts)
	}
}
