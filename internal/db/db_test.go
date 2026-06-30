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

func TestMigrateSeedsCategoriesAndItems(t *testing.T) {
	conn := openTestDB(t)

	var categoryCount int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM categories`).Scan(&categoryCount); err != nil {
		t.Fatalf("query categories: %v", err)
	}
	if categoryCount == 0 {
		t.Fatal("expected seeded categories, got none")
	}

	var itemCount int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM items`).Scan(&itemCount); err != nil {
		t.Fatalf("query items: %v", err)
	}
	if itemCount == 0 {
		t.Fatal("expected seeded items, got none")
	}

	var orphans int
	if err := conn.QueryRow(`
		SELECT COUNT(*) FROM items i
		LEFT JOIN categories c ON c.id = i.category_id
		WHERE c.id IS NULL
	`).Scan(&orphans); err != nil {
		t.Fatalf("query orphaned items: %v", err)
	}
	if orphans != 0 {
		t.Fatalf("found %d items referencing a missing category", orphans)
	}
}

func TestSplitStatements(t *testing.T) {
	script := "CREATE TABLE a (id TEXT);\n\nINSERT INTO a VALUES ('x');\n"
	stmts := splitStatements(script)
	if len(stmts) != 2 {
		t.Fatalf("len(stmts) = %d, want 2: %v", len(stmts), stmts)
	}
}
