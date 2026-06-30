package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/mogensen/lensrace/internal/models"
	"github.com/mogensen/lensrace/internal/server"
)

type sessionBody struct {
	models.GameState
	PlayerID string `json:"playerId"`
}

// doJSON issues a JSON request against app and returns the response status
// code, decoding the body into out if non-nil.
func doJSON(t *testing.T, app *fiber.App, method, path string, body any, out any) int {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return resp.StatusCode
}

func TestCreateGameEndpoint(t *testing.T) {
	app := server.New(newTestApp(t))

	var created sessionBody
	rec := doJSON(t, app, "POST", "/api/games", map[string]any{
		"categoryId": "house-essentials",
		"hostName":   "Alice",
	}, &created)

	if rec != fiber.StatusCreated {
		t.Fatalf("status = %d, want %d", rec, fiber.StatusCreated)
	}
	if created.Game.Status != "waiting" {
		t.Errorf("status = %q, want waiting", created.Game.Status)
	}
	if created.PlayerID == "" {
		t.Error("expected non-empty playerId")
	}
	if len(created.Players) != 1 {
		t.Errorf("players = %d, want 1", len(created.Players))
	}
}

func TestCreateGameEndpointValidation(t *testing.T) {
	app := server.New(newTestApp(t))

	rec := doJSON(t, app, "POST", "/api/games", map[string]any{
		"categoryId": "",
		"hostName":   "Alice",
	}, nil)
	if rec != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec, fiber.StatusBadRequest)
	}
}

func TestFullGameLifecycleEndpoints(t *testing.T) {
	app := server.New(newTestApp(t))

	var created sessionBody
	doJSON(t, app, "POST", "/api/games", map[string]any{
		"categoryId": "house-essentials",
		"hostName":   "Alice",
	}, &created)
	gameID := created.Game.ID
	hostID := created.PlayerID

	var joined sessionBody
	joinRec := doJSON(t, app, "POST", "/api/games/"+gameID+"/join", map[string]any{
		"name": "Bob",
	}, &joined)
	if joinRec != fiber.StatusCreated {
		t.Fatalf("join status = %d, want %d", joinRec, fiber.StatusCreated)
	}
	if len(joined.Players) != 2 {
		t.Fatalf("players after join = %d, want 2", len(joined.Players))
	}

	var afterStart models.GameState
	startRec := doJSON(t, app, "POST", "/api/games/"+gameID+"/start", map[string]any{
		"playerId": hostID,
	}, &afterStart)
	if startRec != fiber.StatusOK {
		t.Fatalf("start status = %d, want %d", startRec, fiber.StatusOK)
	}
	if afterStart.Game.Status != "playing" {
		t.Fatalf("status after start = %q, want playing", afterStart.Game.Status)
	}

	var itemID string
	for _, item := range afterStart.Items {
		if item.CategoryID == "house-essentials" {
			itemID = item.ID
			break
		}
	}
	if itemID == "" {
		t.Fatal("no item found to capture")
	}

	var afterCapture struct {
		models.GameState
		Capture models.Capture `json:"capture"`
	}
	captureRec := doJSON(t, app, "POST", "/api/games/"+gameID+"/captures", map[string]any{
		"playerId": hostID,
		"itemId":   itemID,
	}, &afterCapture)
	if captureRec != fiber.StatusCreated {
		t.Fatalf("capture status = %d, want %d", captureRec, fiber.StatusCreated)
	}
	if afterCapture.Capture.ItemID != itemID {
		t.Errorf("capture itemId = %q, want %q", afterCapture.Capture.ItemID, itemID)
	}

	var hostScore int
	for _, p := range afterCapture.Players {
		if p.ID == hostID {
			hostScore = p.Score
		}
	}
	if hostScore != 1 {
		t.Errorf("host score = %d, want 1", hostScore)
	}

	// Duplicate capture should conflict.
	dupRec := doJSON(t, app, "POST", "/api/games/"+gameID+"/captures", map[string]any{
		"playerId": hostID,
		"itemId":   itemID,
	}, nil)
	if dupRec != fiber.StatusConflict {
		t.Fatalf("duplicate capture status = %d, want %d", dupRec, fiber.StatusConflict)
	}

	// Non-host cannot start a (now-playing) game; also covers already-started.
	startAgainRec := doJSON(t, app, "POST", "/api/games/"+gameID+"/start", map[string]any{
		"playerId": joined.PlayerID,
	}, nil)
	if startAgainRec != fiber.StatusForbidden && startAgainRec != fiber.StatusConflict {
		t.Fatalf("restart status = %d, want 403 or 409", startAgainRec)
	}

	// GET state should be retrievable by join code too.
	var byCode models.GameState
	getRec := doJSON(t, app, "GET", "/api/games/"+afterStart.Game.JoinCode, nil, &byCode)
	if getRec != fiber.StatusOK {
		t.Fatalf("get by join code status = %d, want %d", getRec, fiber.StatusOK)
	}
	if byCode.Game.ID != gameID {
		t.Errorf("got game id %q via join code, want %q", byCode.Game.ID, gameID)
	}
}

func TestGetGameNotFound(t *testing.T) {
	app := server.New(newTestApp(t))

	rec := doJSON(t, app, "GET", "/api/games/does-not-exist", nil, nil)
	if rec != fiber.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec, fiber.StatusNotFound)
	}
}
