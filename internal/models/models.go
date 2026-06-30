// Package models holds the API-facing representations of game entities.
package models

// Game is a single race session.
type Game struct {
	ID              string  `json:"id"`
	JoinCode        string  `json:"joinCode"`
	HostID          *string `json:"hostId,omitempty"`
	CategoryID      string  `json:"categoryId"`
	Status          string  `json:"status"`
	DurationSeconds int     `json:"durationSeconds"`
	CreatedAt       string  `json:"createdAt"`
	StartedAt       *string `json:"startedAt,omitempty"`
	EndedAt         *string `json:"endedAt,omitempty"`
}

// Player is a participant in a game. Score is derived from the player's
// captures, not stored directly.
type Player struct {
	ID             string  `json:"id"`
	GameID         string  `json:"gameId"`
	Name           string  `json:"name"`
	IsHost         bool    `json:"isHost"`
	Score          int     `json:"score"`
	ConnectedAt    string  `json:"connectedAt"`
	DisconnectedAt *string `json:"disconnectedAt,omitempty"`
}

// Item is an object belonging to a category that players try to capture.
type Item struct {
	ID          string `json:"id"`
	CategoryID  string `json:"categoryId"`
	Label       string `json:"label"`
	DisplayName string `json:"displayName"`
}

// Capture records a player successfully photographing an item.
type Capture struct {
	ID         string   `json:"id"`
	GameID     string   `json:"gameId"`
	PlayerID   string   `json:"playerId"`
	ItemID     string   `json:"itemId"`
	Confidence *float64 `json:"confidence,omitempty"`
	CapturedAt string   `json:"capturedAt"`
}

// GameState is the full view of a game returned to clients: the game record
// plus its category items and current players (with derived scores).
type GameState struct {
	Game    Game     `json:"game"`
	Items   []Item   `json:"items"`
	Players []Player `json:"players"`
}
