package handlers_test

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/mogensen/lensrace/internal/db"
	"github.com/mogensen/lensrace/internal/handlers"
	"github.com/mogensen/lensrace/internal/server"
)

func newTestApp(t *testing.T) *sql.DB {
	t.Helper()

	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("db.Migrate: %v", err)
	}
	return conn
}

func TestHealthEndpoint(t *testing.T) {
	app := server.New(newTestApp(t))

	resp, err := app.Test(httptest.NewRequest("GET", "/api/health", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status field = %q, want %q", body["status"], "ok")
	}
}

func TestCategoriesEndpoint(t *testing.T) {
	app := server.New(newTestApp(t))

	resp, err := app.Test(httptest.NewRequest("GET", "/api/categories", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var categories []handlers.Category
	if err := json.NewDecoder(resp.Body).Decode(&categories); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(categories) == 0 {
		t.Fatal("expected at least one seeded category")
	}
	for _, c := range categories {
		if c.ID == "" || c.Name == "" {
			t.Fatalf("category missing id/name: %+v", c)
		}
	}
}
