package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mogensen/lensrace/internal/catalog"
	"github.com/mogensen/lensrace/internal/db"
)

const testCategoryID = "house-essentials"

func newTestStore(t *testing.T) *Store {
	t.Helper()

	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("db.Migrate: %v", err)
	}

	cat, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}

	return New(conn, cat)
}

func mustCreateGame(t *testing.T, s *Store, duration int) (string, string) {
	t.Helper()
	state, hostID, err := s.CreateGame(context.Background(), testCategoryID, "Host", duration)
	if err != nil {
		t.Fatalf("CreateGame: %v", err)
	}
	return state.Game.ID, hostID
}

func TestCreateGame(t *testing.T) {
	s := newTestStore(t)

	state, hostID, err := s.CreateGame(context.Background(), testCategoryID, "Alice", DefaultDurationSeconds)
	if err != nil {
		t.Fatalf("CreateGame: %v", err)
	}

	if state.Game.Status != "waiting" {
		t.Errorf("status = %q, want waiting", state.Game.Status)
	}
	if state.Game.HostID == nil || *state.Game.HostID != hostID {
		t.Errorf("game host_id = %v, want %q", state.Game.HostID, hostID)
	}
	if len(state.Game.JoinCode) != joinCodeLength {
		t.Errorf("join code length = %d, want %d", len(state.Game.JoinCode), joinCodeLength)
	}
	if len(state.Players) != 1 || !state.Players[0].IsHost || state.Players[0].Name != "Alice" {
		t.Errorf("unexpected players: %+v", state.Players)
	}
	if len(state.Items) != TasksPerGame {
		t.Errorf("len(items) = %d, want %d", len(state.Items), TasksPerGame)
	}
}

func TestCreateGameDrawsRandomSubsetOfItemPool(t *testing.T) {
	s := newTestStore(t)

	poolSize := len(s.cat.ItemIDsInCategory(testCategoryID))
	if poolSize <= TasksPerGame {
		t.Fatalf("item pool for %q has %d items, want more than %d to exercise random selection", testCategoryID, poolSize, TasksPerGame)
	}

	seen := map[string]bool{}
	for range 20 {
		state, _, err := s.CreateGame(context.Background(), testCategoryID, "Host", DefaultDurationSeconds)
		if err != nil {
			t.Fatalf("CreateGame: %v", err)
		}
		if len(state.Items) != TasksPerGame {
			t.Fatalf("len(items) = %d, want %d", len(state.Items), TasksPerGame)
		}
		ids := make([]string, len(state.Items))
		for i, it := range state.Items {
			ids[i] = it.ID
		}
		seen[strings.Join(ids, ",")] = true
	}

	if len(seen) < 2 {
		t.Errorf("got the same %d-item selection every time across 20 games; expected variety", TasksPerGame)
	}
}

func TestCreateGameUnknownCategory(t *testing.T) {
	s := newTestStore(t)

	_, _, err := s.CreateGame(context.Background(), "does-not-exist", "Alice", DefaultDurationSeconds)
	if !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("err = %v, want ErrCategoryNotFound", err)
	}
}

func TestJoinGameByIDAndByJoinCode(t *testing.T) {
	s := newTestStore(t)
	gameID, _ := mustCreateGame(t, s, DefaultDurationSeconds)

	stateByID, p1, err := s.JoinGame(context.Background(), gameID, "Bob")
	if err != nil {
		t.Fatalf("JoinGame by id: %v", err)
	}
	if len(stateByID.Players) != 2 {
		t.Fatalf("players after first join = %d, want 2", len(stateByID.Players))
	}

	stateByCode, p2, err := s.JoinGame(context.Background(), stateByID.Game.JoinCode, "Cara")
	if err != nil {
		t.Fatalf("JoinGame by join code: %v", err)
	}
	if len(stateByCode.Players) != 3 {
		t.Fatalf("players after second join = %d, want 3", len(stateByCode.Players))
	}
	if p1 == p2 {
		t.Error("expected distinct player IDs")
	}

	// lowercase join code should also resolve
	if _, _, err := s.JoinGame(context.Background(), toLower(stateByID.Game.JoinCode), "Dee"); err != nil {
		t.Fatalf("JoinGame with lowercase code: %v", err)
	}
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

func TestJoinGameNotFound(t *testing.T) {
	s := newTestStore(t)
	_, _, err := s.JoinGame(context.Background(), "nonexistent", "Bob")
	if !errors.Is(err, ErrGameNotFound) {
		t.Fatalf("err = %v, want ErrGameNotFound", err)
	}
}

func TestJoinGameAlreadyStarted(t *testing.T) {
	s := newTestStore(t)
	gameID, hostID := mustCreateGame(t, s, DefaultDurationSeconds)

	if _, err := s.StartGame(context.Background(), gameID, hostID); err != nil {
		t.Fatalf("StartGame: %v", err)
	}

	_, _, err := s.JoinGame(context.Background(), gameID, "Late")
	if !errors.Is(err, ErrGameNotWaiting) {
		t.Fatalf("err = %v, want ErrGameNotWaiting", err)
	}
}

func TestStartGameRequiresHost(t *testing.T) {
	s := newTestStore(t)
	gameID, _ := mustCreateGame(t, s, DefaultDurationSeconds)

	_, _, joinErr := s.JoinGame(context.Background(), gameID, "Bob")
	if joinErr != nil {
		t.Fatalf("JoinGame: %v", joinErr)
	}
	state, _, err := s.JoinGame(context.Background(), gameID, "Cara")
	if err != nil {
		t.Fatalf("JoinGame: %v", err)
	}

	var nonHostID string
	for _, p := range state.Players {
		if !p.IsHost {
			nonHostID = p.ID
			break
		}
	}

	_, err = s.StartGame(context.Background(), gameID, nonHostID)
	if !errors.Is(err, ErrNotHost) {
		t.Fatalf("err = %v, want ErrNotHost", err)
	}
}

func TestStartGameTwiceFails(t *testing.T) {
	s := newTestStore(t)
	gameID, hostID := mustCreateGame(t, s, DefaultDurationSeconds)

	if _, err := s.StartGame(context.Background(), gameID, hostID); err != nil {
		t.Fatalf("first StartGame: %v", err)
	}
	_, err := s.StartGame(context.Background(), gameID, hostID)
	if !errors.Is(err, ErrGameNotWaiting) {
		t.Fatalf("err = %v, want ErrGameNotWaiting", err)
	}
}

func TestSetCategoryUpdatesItems(t *testing.T) {
	s := newTestStore(t)
	gameID, hostID := mustCreateGame(t, s, DefaultDurationSeconds)

	state, err := s.SetCategory(context.Background(), gameID, hostID, "city-scavenger")
	if err != nil {
		t.Fatalf("SetCategory: %v", err)
	}
	if state.Game.CategoryID != "city-scavenger" {
		t.Fatalf("category = %q, want city-scavenger", state.Game.CategoryID)
	}
	for _, item := range state.Items {
		if item.CategoryID != "city-scavenger" {
			t.Fatalf("item %s belongs to %q, want city-scavenger", item.ID, item.CategoryID)
		}
	}
}

func TestSetCategoryRequiresHost(t *testing.T) {
	s := newTestStore(t)
	gameID, _ := mustCreateGame(t, s, DefaultDurationSeconds)

	_, _, err := s.JoinGame(context.Background(), gameID, "Bob")
	if err != nil {
		t.Fatalf("JoinGame: %v", err)
	}
	state, err := s.GetGameState(context.Background(), gameID)
	if err != nil {
		t.Fatalf("GetGameState: %v", err)
	}
	var guestID string
	for _, p := range state.Players {
		if !p.IsHost {
			guestID = p.ID
		}
	}

	_, err = s.SetCategory(context.Background(), gameID, guestID, "city-scavenger")
	if !errors.Is(err, ErrNotHost) {
		t.Fatalf("err = %v, want ErrNotHost", err)
	}
}

func TestSetCategoryRejectsAfterStart(t *testing.T) {
	s := newTestStore(t)
	gameID, hostID := mustCreateGame(t, s, DefaultDurationSeconds)
	if _, err := s.StartGame(context.Background(), gameID, hostID); err != nil {
		t.Fatalf("StartGame: %v", err)
	}

	_, err := s.SetCategory(context.Background(), gameID, hostID, "city-scavenger")
	if !errors.Is(err, ErrGameNotWaiting) {
		t.Fatalf("err = %v, want ErrGameNotWaiting", err)
	}
}

func TestSetCategoryUnknownCategory(t *testing.T) {
	s := newTestStore(t)
	gameID, hostID := mustCreateGame(t, s, DefaultDurationSeconds)

	_, err := s.SetCategory(context.Background(), gameID, hostID, "does-not-exist")
	if !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("err = %v, want ErrCategoryNotFound", err)
	}
}

func TestSetDurationUpdatesGame(t *testing.T) {
	s := newTestStore(t)
	gameID, hostID := mustCreateGame(t, s, DefaultDurationSeconds)

	state, err := s.SetDuration(context.Background(), gameID, hostID, 180)
	if err != nil {
		t.Fatalf("SetDuration: %v", err)
	}
	if state.Game.DurationSeconds != 180 {
		t.Fatalf("durationSeconds = %d, want 180", state.Game.DurationSeconds)
	}
}

func TestSetDurationRequiresHost(t *testing.T) {
	s := newTestStore(t)
	gameID, _ := mustCreateGame(t, s, DefaultDurationSeconds)

	_, _, err := s.JoinGame(context.Background(), gameID, "Bob")
	if err != nil {
		t.Fatalf("JoinGame: %v", err)
	}
	state, err := s.GetGameState(context.Background(), gameID)
	if err != nil {
		t.Fatalf("GetGameState: %v", err)
	}
	var guestID string
	for _, p := range state.Players {
		if !p.IsHost {
			guestID = p.ID
		}
	}

	_, err = s.SetDuration(context.Background(), gameID, guestID, 180)
	if !errors.Is(err, ErrNotHost) {
		t.Fatalf("err = %v, want ErrNotHost", err)
	}
}

func TestSetDurationRejectsAfterStart(t *testing.T) {
	s := newTestStore(t)
	gameID, hostID := mustCreateGame(t, s, DefaultDurationSeconds)
	if _, err := s.StartGame(context.Background(), gameID, hostID); err != nil {
		t.Fatalf("StartGame: %v", err)
	}

	_, err := s.SetDuration(context.Background(), gameID, hostID, 180)
	if !errors.Is(err, ErrGameNotWaiting) {
		t.Fatalf("err = %v, want ErrGameNotWaiting", err)
	}
}

func TestRecordCaptureRequiresPlayingGame(t *testing.T) {
	s := newTestStore(t)
	gameID, hostID := mustCreateGame(t, s, DefaultDurationSeconds)
	state, err := s.GetGameState(context.Background(), gameID)
	if err != nil {
		t.Fatalf("GetGameState: %v", err)
	}

	_, _, err = s.RecordCapture(context.Background(), gameID, hostID, state.Items[0].ID, nil)
	if !errors.Is(err, ErrGameNotPlaying) {
		t.Fatalf("err = %v, want ErrGameNotPlaying", err)
	}
}

func TestRecordCaptureScoresAndRejectsDuplicates(t *testing.T) {
	s := newTestStore(t)
	gameID, hostID := mustCreateGame(t, s, DefaultDurationSeconds)
	state, err := s.StartGame(context.Background(), gameID, hostID)
	if err != nil {
		t.Fatalf("StartGame: %v", err)
	}
	itemID := state.Items[0].ID

	confidence := 0.92
	state, capture, err := s.RecordCapture(context.Background(), gameID, hostID, itemID, &confidence)
	if err != nil {
		t.Fatalf("RecordCapture: %v", err)
	}
	if capture.ItemID != itemID || capture.Confidence == nil || *capture.Confidence != confidence {
		t.Errorf("unexpected capture: %+v", capture)
	}

	var hostScore int
	var hostCaptured []string
	for _, p := range state.Players {
		if p.ID == hostID {
			hostScore = p.Score
			hostCaptured = p.CapturedItemIDs
		}
	}
	if hostScore != 1 {
		t.Errorf("host score = %d, want 1", hostScore)
	}
	if len(hostCaptured) != 1 || hostCaptured[0] != itemID {
		t.Errorf("host capturedItemIds = %v, want [%s]", hostCaptured, itemID)
	}

	_, _, err = s.RecordCapture(context.Background(), gameID, hostID, itemID, nil)
	if !errors.Is(err, ErrAlreadyCaptured) {
		t.Fatalf("err = %v, want ErrAlreadyCaptured", err)
	}
}

func TestRecordCaptureRejectsItemFromOtherCategory(t *testing.T) {
	s := newTestStore(t)
	gameID, hostID := mustCreateGame(t, s, DefaultDurationSeconds)
	if _, err := s.StartGame(context.Background(), gameID, hostID); err != nil {
		t.Fatalf("StartGame: %v", err)
	}

	_, _, err := s.RecordCapture(context.Background(), gameID, hostID, "city-car", nil)
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("err = %v, want ErrItemNotFound", err)
	}
}

func TestRecordCaptureRejectsPlayerFromOtherGame(t *testing.T) {
	s := newTestStore(t)
	gameID, hostID := mustCreateGame(t, s, DefaultDurationSeconds)
	state, err := s.StartGame(context.Background(), gameID, hostID)
	if err != nil {
		t.Fatalf("StartGame: %v", err)
	}

	otherGameID, otherHostID := mustCreateGame(t, s, DefaultDurationSeconds)
	if _, err := s.StartGame(context.Background(), otherGameID, otherHostID); err != nil {
		t.Fatalf("StartGame other: %v", err)
	}

	_, _, err = s.RecordCapture(context.Background(), gameID, otherHostID, state.Items[0].ID, nil)
	if !errors.Is(err, ErrPlayerNotInGame) {
		t.Fatalf("err = %v, want ErrPlayerNotInGame", err)
	}
}

func TestRecordCaptureFinishesGameOnFullCompletion(t *testing.T) {
	s := newTestStore(t)
	gameID, hostID := mustCreateGame(t, s, DefaultDurationSeconds)
	state, err := s.StartGame(context.Background(), gameID, hostID)
	if err != nil {
		t.Fatalf("StartGame: %v", err)
	}

	finalState := state
	for _, item := range state.Items {
		finalState, _, err = s.RecordCapture(context.Background(), gameID, hostID, item.ID, nil)
		if err != nil {
			t.Fatalf("RecordCapture(%s): %v", item.ID, err)
		}
	}

	if finalState.Game.Status != "finished" {
		t.Errorf("status = %q, want finished", finalState.Game.Status)
	}
	if finalState.Game.EndedAt == nil {
		t.Error("expected ended_at to be set")
	}

	// Game is finished, so further captures must be rejected even though
	// every item has already been captured by the host.
	_, _, err = s.RecordCapture(context.Background(), gameID, hostID, state.Items[0].ID, nil)
	if !errors.Is(err, ErrGameNotPlaying) && !errors.Is(err, ErrAlreadyCaptured) {
		t.Fatalf("err = %v, want ErrGameNotPlaying or ErrAlreadyCaptured", err)
	}
}

func TestExpireIfNeededTransitionsToFinished(t *testing.T) {
	s := newTestStore(t)
	gameID, hostID := mustCreateGame(t, s, MinDurationSeconds)
	if _, err := s.StartGame(context.Background(), gameID, hostID); err != nil {
		t.Fatalf("StartGame: %v", err)
	}

	// Force started_at far enough in the past that the game has expired.
	if _, err := s.db.ExecContext(context.Background(), `
		UPDATE games SET started_at = '2000-01-01T00:00:00.000Z' WHERE id = ?
	`, gameID); err != nil {
		t.Fatalf("force expiry: %v", err)
	}

	state, err := s.GetGameState(context.Background(), gameID)
	if err != nil {
		t.Fatalf("GetGameState: %v", err)
	}
	if state.Game.Status != "finished" {
		t.Errorf("status = %q, want finished", state.Game.Status)
	}

	_, _, err = s.RecordCapture(context.Background(), gameID, hostID, "house-chair", nil)
	if !errors.Is(err, ErrGameNotPlaying) {
		t.Fatalf("err = %v, want ErrGameNotPlaying", err)
	}
}
