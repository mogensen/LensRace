package handlers_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/mogensen/lensrace/internal/models"
	"github.com/mogensen/lensrace/internal/realtime"
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

// postJSON issues a real HTTP POST against baseURL (used instead of doJSON
// when a test also exercises a live net.Listener, since mixing app.Test()
// with app.Listener() on the same app races on fasthttp's internal state).
func postJSON(t *testing.T, baseURL, path string, body any, out any) int {
	t.Helper()

	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	resp, err := http.Post(baseURL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("http.Post %s: %v", path, err)
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
	app := server.New(newTestApp(t), realtime.New())

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
	app := server.New(newTestApp(t), realtime.New())

	rec := doJSON(t, app, "POST", "/api/games", map[string]any{
		"categoryId": "",
		"hostName":   "Alice",
	}, nil)
	if rec != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec, fiber.StatusBadRequest)
	}
}

func TestFullGameLifecycleEndpoints(t *testing.T) {
	app := server.New(newTestApp(t), realtime.New())

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

func TestSetCategoryEndpoint(t *testing.T) {
	app := server.New(newTestApp(t), realtime.New())

	var created sessionBody
	doJSON(t, app, "POST", "/api/games", map[string]any{
		"categoryId": "house-essentials",
		"hostName":   "Alice",
	}, &created)
	gameID := created.Game.ID
	hostID := created.PlayerID

	var updated models.GameState
	rec := doJSON(t, app, "PATCH", "/api/games/"+gameID+"/category", map[string]any{
		"playerId":   hostID,
		"categoryId": "city-scavenger",
	}, &updated)
	if rec != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", rec, fiber.StatusOK)
	}
	if updated.Game.CategoryID != "city-scavenger" {
		t.Fatalf("categoryId = %q, want city-scavenger", updated.Game.CategoryID)
	}
	for _, item := range updated.Items {
		if item.CategoryID != "city-scavenger" {
			t.Fatalf("item %s belongs to %q, want city-scavenger", item.ID, item.CategoryID)
		}
	}

	forbidden := doJSON(t, app, "PATCH", "/api/games/"+gameID+"/category", map[string]any{
		"playerId":   "not-a-player",
		"categoryId": "house-essentials",
	}, nil)
	if forbidden != fiber.StatusForbidden {
		t.Fatalf("status = %d, want %d", forbidden, fiber.StatusForbidden)
	}
}

func TestGetGameNotFound(t *testing.T) {
	app := server.New(newTestApp(t), realtime.New())

	rec := doJSON(t, app, "GET", "/api/games/does-not-exist", nil, nil)
	if rec != fiber.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec, fiber.StatusNotFound)
	}
}

func TestEventsEndpointStreamsStateChanges(t *testing.T) {
	app := server.New(newTestApp(t), realtime.New())

	// app.Test() can't exercise a long-lived SSE stream: it feeds the
	// request through an in-memory testConn and blocks until
	// fasthttp.Server.ServeConn fully returns, which never happens while
	// our handler keeps the connection open. A real listener gives genuine
	// concurrent client/server I/O instead. Every request in this test goes
	// over that real listener (not app.Test()) — fasthttp.Server shares
	// internal state between its Serve() and ServeConn() entry points that
	// isn't safe to use concurrently, so mixing the two on one app races.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- app.Listener(ln) }()
	t.Cleanup(func() {
		// A plain Shutdown() blocks indefinitely on any still-open SSE
		// connection (it never notices the client closed until the next
		// write attempt, which is up to sseHeartbeatInterval away). Bound
		// it so test cleanup can't hang.
		_ = app.ShutdownWithTimeout(time.Second)
		<-serveErr
	})
	baseURL := "http://" + ln.Addr().String()

	var created sessionBody
	postJSON(t, baseURL, "/api/games", map[string]any{
		"categoryId": "house-essentials",
		"hostName":   "Alice",
	}, &created)
	gameID := created.Game.ID

	resp, err := http.Get(baseURL + "/api/games/" + gameID + "/events")
	if err != nil {
		t.Fatalf("http.Get events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", resp.Header.Get("Content-Type"))
	}

	events := make(chan models.GameState, 4)
	errs := make(chan error, 1)
	go readSSEStates(resp.Body, events, errs)

	first := waitForSSEState(t, events, errs)
	if first.Game.Status != "waiting" {
		t.Fatalf("first event status = %q, want waiting", first.Game.Status)
	}
	if len(first.Players) != 1 {
		t.Fatalf("first event players = %d, want 1", len(first.Players))
	}

	joinRec := postJSON(t, baseURL, "/api/games/"+gameID+"/join", map[string]any{"name": "Bob"}, nil)
	if joinRec != fiber.StatusCreated {
		t.Fatalf("join status = %d, want %d", joinRec, fiber.StatusCreated)
	}

	second := waitForSSEState(t, events, errs)
	if len(second.Players) != 2 {
		t.Fatalf("players after join via SSE = %d, want 2", len(second.Players))
	}
}

// readSSEStates parses "data: <json>" lines from an SSE body and decodes
// each as a models.GameState, sending results on out until the stream ends
// or an error/close occurs (sent on errs).
func readSSEStates(body io.ReadCloser, out chan<- models.GameState, errs chan<- error) {
	reader := bufio.NewReader(body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			errs <- err
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var state models.GameState
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &state); err != nil {
			errs <- err
			return
		}
		out <- state
	}
}

func waitForSSEState(t *testing.T, events <-chan models.GameState, errs <-chan error) models.GameState {
	t.Helper()
	select {
	case state := <-events:
		return state
	case err := <-errs:
		t.Fatalf("reading SSE stream: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE event")
	}
	return models.GameState{}
}
