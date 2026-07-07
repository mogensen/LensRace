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

// cocoSsdClasses is the fixed, closed set of object classes the frontend's
// on-device detector (TensorFlow.js COCO-SSD) can ever recognize — see
// coco-ssd's classes.ts. An item's label that isn't in this list can never
// be auto-captured, no matter what the player points the camera at, so
// every seeded item must use one of these exact strings.
var cocoSsdClasses = map[string]bool{
	"person": true, "bicycle": true, "car": true, "motorcycle": true, "airplane": true,
	"bus": true, "train": true, "truck": true, "boat": true, "traffic light": true,
	"fire hydrant": true, "stop sign": true, "parking meter": true, "bench": true, "bird": true,
	"cat": true, "dog": true, "horse": true, "sheep": true, "cow": true,
	"elephant": true, "bear": true, "zebra": true, "giraffe": true, "backpack": true,
	"umbrella": true, "handbag": true, "tie": true, "suitcase": true, "frisbee": true,
	"skis": true, "snowboard": true, "sports ball": true, "kite": true, "baseball bat": true,
	"baseball glove": true, "skateboard": true, "surfboard": true, "tennis racket": true, "bottle": true,
	"wine glass": true, "cup": true, "fork": true, "knife": true, "spoon": true,
	"bowl": true, "banana": true, "apple": true, "sandwich": true, "orange": true,
	"broccoli": true, "carrot": true, "hot dog": true, "pizza": true, "donut": true,
	"cake": true, "chair": true, "couch": true, "potted plant": true, "bed": true,
	"dining table": true, "toilet": true, "tv": true, "laptop": true, "mouse": true,
	"remote": true, "keyboard": true, "cell phone": true, "microwave": true, "oven": true,
	"toaster": true, "sink": true, "refrigerator": true, "book": true, "clock": true,
	"vase": true, "scissors": true, "teddy bear": true, "hair drier": true, "toothbrush": true,
}

func TestSeededItemLabelsAreDetectableByCocoSsd(t *testing.T) {
	conn := openTestDB(t)

	rows, err := conn.Query(`SELECT id, label FROM items`)
	if err != nil {
		t.Fatalf("query items: %v", err)
	}
	defer rows.Close()

	var undetectable []string
	for rows.Next() {
		var id, label string
		if err := rows.Scan(&id, &label); err != nil {
			t.Fatalf("scan item: %v", err)
		}
		if !cocoSsdClasses[label] {
			undetectable = append(undetectable, id+" (label="+label+")")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate items: %v", err)
	}
	if len(undetectable) > 0 {
		t.Fatalf("items with a label the on-device detector can never match: %v", undetectable)
	}
}

func TestForestAndCampLifeHaveAtLeast30Items(t *testing.T) {
	conn := openTestDB(t)

	for _, categoryID := range []string{"forest", "camp-life"} {
		var count int
		if err := conn.QueryRow(`SELECT COUNT(*) FROM items WHERE category_id = ?`, categoryID).Scan(&count); err != nil {
			t.Fatalf("count items for %s: %v", categoryID, err)
		}
		if count < 30 {
			t.Errorf("category %s has %d items, want at least 30", categoryID, count)
		}
	}
}

func TestSplitStatements(t *testing.T) {
	script := "CREATE TABLE a (id TEXT);\n\nINSERT INTO a VALUES ('x');\n"
	stmts := splitStatements(script)
	if len(stmts) != 2 {
		t.Fatalf("len(stmts) = %d, want 2: %v", len(stmts), stmts)
	}
}
