package realtime

import (
	"context"
	"log"
	"time"

	"github.com/mogensen/lensrace/internal/store"
)

// WatchExpirations periodically scans the hub's cached 'playing' games for
// ones whose duration has elapsed. The deadline check itself is in-memory
// and free; only games actually past their deadline trigger a database
// round trip (via the store, which performs the authoritative transition
// to 'finished'), and the refreshed state is republished so connected SSE
// clients see the status change without polling. It blocks until ctx is
// cancelled.
func WatchExpirations(ctx context.Context, hub *Hub, st *store.Store, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkExpirations(ctx, hub, st)
		}
	}
}

func checkExpirations(ctx context.Context, hub *Hub, st *store.Store) {
	now := time.Now().UTC()

	for _, cached := range hub.PlayingGames() {
		if cached.Game.StartedAt == nil {
			continue
		}
		started, err := time.Parse(store.TimeLayout, *cached.Game.StartedAt)
		if err != nil {
			continue
		}
		deadline := started.Add(time.Duration(cached.Game.DurationSeconds) * time.Second)
		if now.Before(deadline) {
			continue
		}

		state, err := st.GetGameState(ctx, cached.Game.ID)
		if err != nil {
			log.Printf("realtime: refresh expired game %s: %v", cached.Game.ID, err)
			continue
		}
		hub.Publish(cached.Game.ID, *state)
	}
}
