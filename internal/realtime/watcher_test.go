package realtime

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/mogensen/lensrace/internal/db"
	"github.com/mogensen/lensrace/internal/store"
)

func newTestStore(t *testing.T) (*store.Store, *sql.DB) {
	t.Helper()

	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := db.Migrate(conn); err != nil {
		t.Fatalf("db.Migrate: %v", err)
	}
	return store.New(conn), conn
}

func TestCheckExpirationsFinishesElapsedGames(t *testing.T) {
	ctx := context.Background()
	st, conn := newTestStore(t)
	h := New()

	state, hostID, err := st.CreateGame(ctx, "house-essentials", "Alice", store.MinDurationSeconds)
	if err != nil {
		t.Fatalf("CreateGame: %v", err)
	}
	state, err = st.StartGame(ctx, state.Game.ID, hostID)
	if err != nil {
		t.Fatalf("StartGame: %v", err)
	}

	// Backdate started_at in the database (the authoritative source) so the
	// game is genuinely past its deadline, then seed the cache to match —
	// as if it had been cached just before the deadline passed and nobody
	// has polled GetGameState since. That gap is exactly what
	// WatchExpirations exists to close.
	past := time.Now().UTC().Add(-time.Hour).Format(store.TimeLayout)
	if _, err := conn.ExecContext(ctx, `UPDATE games SET started_at = ? WHERE id = ?`, past, state.Game.ID); err != nil {
		t.Fatalf("backdate started_at: %v", err)
	}
	cached := *state
	cached.Game.StartedAt = &past
	h.Publish(state.Game.ID, cached)

	checkExpirations(ctx, h, st)

	got, ok := h.Get(state.Game.ID)
	if !ok {
		t.Fatal("expected cached state after checkExpirations")
	}
	if got.Game.Status != "finished" {
		t.Errorf("status = %q, want finished", got.Game.Status)
	}
}

func TestCheckExpirationsIgnoresGamesNotYetDue(t *testing.T) {
	ctx := context.Background()
	st, _ := newTestStore(t)
	h := New()

	state, hostID, err := st.CreateGame(ctx, "house-essentials", "Alice", store.DefaultDurationSeconds)
	if err != nil {
		t.Fatalf("CreateGame: %v", err)
	}
	state, err = st.StartGame(ctx, state.Game.ID, hostID)
	if err != nil {
		t.Fatalf("StartGame: %v", err)
	}
	h.Publish(state.Game.ID, *state)

	checkExpirations(ctx, h, st)

	got, ok := h.Get(state.Game.ID)
	if !ok {
		t.Fatal("expected cached state")
	}
	if got.Game.Status != "playing" {
		t.Errorf("status = %q, want playing (not yet due)", got.Game.Status)
	}
}

func TestWatchExpirationsStopsOnContextCancel(t *testing.T) {
	st, _ := newTestStore(t)
	h := New()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		WatchExpirations(ctx, h, st, 10*time.Millisecond)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("WatchExpirations did not return after context cancellation")
	}
}
