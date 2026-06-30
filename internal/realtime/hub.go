// Package realtime caches the latest known state for each in-progress game
// and fans out updates to connected SSE subscribers, so reads and pushes
// don't need a database round trip on every request.
package realtime

import (
	"sync"

	"github.com/mogensen/lensrace/internal/models"
)

// Hub holds one topic per game.
type Hub struct {
	mu     sync.RWMutex
	topics map[string]*topic
}

type topic struct {
	mu          sync.RWMutex
	state       models.GameState
	has         bool
	subscribers map[chan models.GameState]struct{}
}

// New creates an empty Hub.
func New() *Hub {
	return &Hub{topics: make(map[string]*topic)}
}

func (h *Hub) topicFor(gameID string) *topic {
	h.mu.Lock()
	defer h.mu.Unlock()

	t, ok := h.topics[gameID]
	if !ok {
		t = &topic{subscribers: make(map[chan models.GameState]struct{})}
		h.topics[gameID] = t
	}
	return t
}

// Get returns the cached state for a game, if any has been published yet.
func (h *Hub) Get(gameID string) (models.GameState, bool) {
	h.mu.RLock()
	t, ok := h.topics[gameID]
	h.mu.RUnlock()
	if !ok {
		return models.GameState{}, false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state, t.has
}

// Publish updates the cached state for a game and notifies all current
// subscribers. Each subscriber channel is buffered for exactly one pending
// update; a slow subscriber's stale pending update is replaced rather than
// queued, so subscribers always see the latest state instead of blocking
// the publisher or falling behind on a backlog.
func (h *Hub) Publish(gameID string, state models.GameState) {
	t := h.topicFor(gameID)

	t.mu.Lock()
	t.state = state
	t.has = true
	subs := make([]chan models.GameState, 0, len(t.subscribers))
	for ch := range t.subscribers {
		subs = append(subs, ch)
	}
	t.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- state:
		default:
			select {
			case <-ch:
			default:
			}
			select {
			case ch <- state:
			default:
			}
		}
	}
}

// Subscribe registers a new listener for a game's state updates. The
// returned channel receives the current cached state immediately (if any)
// and every subsequent update. The caller must invoke the returned
// unsubscribe function when it stops listening.
func (h *Hub) Subscribe(gameID string) (<-chan models.GameState, func()) {
	t := h.topicFor(gameID)
	ch := make(chan models.GameState, 1)

	t.mu.Lock()
	t.subscribers[ch] = struct{}{}
	if t.has {
		ch <- t.state
	}
	t.mu.Unlock()

	unsubscribe := func() {
		t.mu.Lock()
		delete(t.subscribers, ch)
		t.mu.Unlock()
	}
	return ch, unsubscribe
}

// PlayingGames returns the cached state of every game currently cached as
// 'playing', for the expiry watcher to scan without hitting the database.
func (h *Hub) PlayingGames() []models.GameState {
	h.mu.RLock()
	defer h.mu.RUnlock()

	games := make([]models.GameState, 0, len(h.topics))
	for _, t := range h.topics {
		t.mu.RLock()
		if t.has && t.state.Game.Status == "playing" {
			games = append(games, t.state)
		}
		t.mu.RUnlock()
	}
	return games
}
