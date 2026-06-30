package realtime

import (
	"testing"
	"time"

	"github.com/mogensen/lensrace/internal/models"
)

func gameState(id, status string) models.GameState {
	return models.GameState{Game: models.Game{ID: id, Status: status}}
}

func TestGetMissReturnsFalse(t *testing.T) {
	h := New()
	if _, ok := h.Get("missing"); ok {
		t.Fatal("expected ok=false for an unpublished game")
	}
}

func TestPublishThenGet(t *testing.T) {
	h := New()
	h.Publish("g1", gameState("g1", "waiting"))

	state, ok := h.Get("g1")
	if !ok {
		t.Fatal("expected ok=true after Publish")
	}
	if state.Game.Status != "waiting" {
		t.Errorf("status = %q, want waiting", state.Game.Status)
	}
}

func TestSubscribeReceivesCachedStateImmediately(t *testing.T) {
	h := New()
	h.Publish("g1", gameState("g1", "playing"))

	ch, unsubscribe := h.Subscribe("g1")
	defer unsubscribe()

	select {
	case state := <-ch:
		if state.Game.Status != "playing" {
			t.Errorf("status = %q, want playing", state.Game.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for cached state")
	}
}

func TestSubscribeReceivesSubsequentPublishes(t *testing.T) {
	h := New()
	ch, unsubscribe := h.Subscribe("g1")
	defer unsubscribe()

	// Publishes are consumed between each send so they can't coalesce in
	// the channel's single-slot buffer (see TestSlowSubscriberSeesLatestStateNotBacklog
	// for the coalescing behavior when nobody reads in between).
	h.Publish("g1", gameState("g1", "waiting"))
	first := recvOrFail(t, ch)
	if first.Game.Status != "waiting" {
		t.Errorf("first status = %q, want waiting", first.Game.Status)
	}

	h.Publish("g1", gameState("g1", "playing"))
	second := recvOrFail(t, ch)
	if second.Game.Status != "playing" {
		t.Errorf("second status = %q, want playing", second.Game.Status)
	}
}

func TestSlowSubscriberSeesLatestStateNotBacklog(t *testing.T) {
	h := New()
	ch, unsubscribe := h.Subscribe("g1")
	defer unsubscribe()

	// Publish faster than the (unbuffered-consumer) subscriber reads, with
	// no one draining the channel in between.
	h.Publish("g1", gameState("g1", "waiting"))
	h.Publish("g1", gameState("g1", "countdown"))
	h.Publish("g1", gameState("g1", "playing"))
	h.Publish("g1", gameState("g1", "finished"))

	got := recvOrFail(t, ch)
	if got.Game.Status != "finished" {
		t.Errorf("status = %q, want finished (latest state wins)", got.Game.Status)
	}

	select {
	case extra := <-ch:
		t.Fatalf("expected no further buffered events, got %+v", extra)
	default:
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	h := New()
	ch, unsubscribe := h.Subscribe("g1")
	unsubscribe()

	h.Publish("g1", gameState("g1", "playing"))

	select {
	case state, ok := <-ch:
		if ok {
			t.Fatalf("expected no delivery after unsubscribe, got %+v", state)
		}
	case <-time.After(50 * time.Millisecond):
		// No delivery, as expected.
	}
}

func TestPlayingGamesFiltersByStatus(t *testing.T) {
	h := New()
	h.Publish("g1", gameState("g1", "waiting"))
	h.Publish("g2", gameState("g2", "playing"))
	h.Publish("g3", gameState("g3", "finished"))

	playing := h.PlayingGames()
	if len(playing) != 1 || playing[0].Game.ID != "g2" {
		t.Fatalf("PlayingGames = %+v, want only g2", playing)
	}
}

func recvOrFail(t *testing.T, ch <-chan models.GameState) models.GameState {
	t.Helper()
	select {
	case state := <-ch:
		return state
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
		return models.GameState{}
	}
}
