// Package store implements the game lifecycle — creating, joining,
// starting, and scoring games — on top of the schema in internal/db.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	mathrand "math/rand/v2"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mogensen/lensrace/internal/models"
)

var (
	ErrCategoryNotFound = errors.New("category not found")
	ErrGameNotFound     = errors.New("game not found")
	ErrPlayerNotFound   = errors.New("player not found")
	ErrItemNotFound     = errors.New("item not found")
	ErrGameNotWaiting   = errors.New("game has already started")
	ErrGameNotPlaying   = errors.New("game is not in progress")
	ErrNotHost          = errors.New("only the host can perform this action")
	ErrAlreadyCaptured  = errors.New("item already captured")
	ErrPlayerNotInGame  = errors.New("player is not part of this game")
)

const (
	MinDurationSeconds     = 30
	MaxDurationSeconds     = 3600
	DefaultDurationSeconds = 300

	// TasksPerGame is how many items are randomly drawn from a category's
	// item pool for a single game. If the category has fewer items than
	// this, every item in the pool is used.
	TasksPerGame = 6

	// TimeLayout matches the strftime format used by SQLite column defaults
	// ('%Y-%m-%dT%H:%M:%fZ') so app-generated and DB-generated timestamps
	// sort and parse identically.
	TimeLayout = "2006-01-02T15:04:05.000Z"

	joinCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/1/I
	joinCodeLength   = 6
)

// Store provides game-lifecycle operations backed by SQLite.
type Store struct {
	db *sql.DB
}

// New builds a Store over an already-migrated database connection.
func New(conn *sql.DB) *Store {
	return &Store{db: conn}
}

// queryer is satisfied by both *sql.DB and *sql.Tx so helpers can run
// inside or outside a transaction without duplicating queries.
type queryer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func now() string {
	return time.Now().UTC().Format(TimeLayout)
}

// CreateGame creates a new game in the 'waiting' state along with its host
// player, and returns the resulting state plus the host's player ID.
func (s *Store) CreateGame(ctx context.Context, categoryID, hostName string, durationSeconds int) (*models.GameState, string, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE id = ?`, categoryID).Scan(&exists); err != nil {
		return nil, "", fmt.Errorf("check category: %w", err)
	}
	if exists == 0 {
		return nil, "", ErrCategoryNotFound
	}

	joinCode, err := s.uniqueJoinCode(ctx)
	if err != nil {
		return nil, "", err
	}

	gameID := uuid.NewString()
	playerID := uuid.NewString()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// host_id starts NULL to break the circular FK with players, then gets
	// backfilled once the host player row exists.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO games (id, join_code, host_id, category_id, status, duration_seconds)
		VALUES (?, ?, NULL, ?, 'waiting', ?)
	`, gameID, joinCode, categoryID, durationSeconds); err != nil {
		return nil, "", fmt.Errorf("insert game: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO players (id, game_id, name, is_host) VALUES (?, ?, ?, TRUE)
	`, playerID, gameID, hostName); err != nil {
		return nil, "", fmt.Errorf("insert host player: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE games SET host_id = ? WHERE id = ?`, playerID, gameID); err != nil {
		return nil, "", fmt.Errorf("set host: %w", err)
	}

	if err := selectGameItems(ctx, tx, gameID, categoryID); err != nil {
		return nil, "", err
	}

	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("commit: %w", err)
	}

	state, err := s.getGameStateByID(ctx, gameID)
	if err != nil {
		return nil, "", err
	}
	return state, playerID, nil
}

// GetGameState returns the full state of a game, looked up by either its
// internal ID or its public join code.
func (s *Store) GetGameState(ctx context.Context, idOrCode string) (*models.GameState, error) {
	id, err := resolveGameID(ctx, s.db, idOrCode)
	if err != nil {
		return nil, err
	}
	return s.getGameStateByID(ctx, id)
}

// JoinGame adds a new (non-host) player to a waiting game.
func (s *Store) JoinGame(ctx context.Context, idOrCode, name string) (*models.GameState, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	id, err := resolveGameID(ctx, tx, idOrCode)
	if err != nil {
		return nil, "", err
	}
	game, err := loadGame(ctx, tx, id)
	if err != nil {
		return nil, "", err
	}
	if err := expireIfNeeded(ctx, tx, game); err != nil {
		return nil, "", err
	}
	if game.Status != "waiting" {
		return nil, "", ErrGameNotWaiting
	}

	playerID := uuid.NewString()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO players (id, game_id, name, is_host) VALUES (?, ?, ?, FALSE)
	`, playerID, id, name); err != nil {
		return nil, "", fmt.Errorf("insert player: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, "", fmt.Errorf("commit: %w", err)
	}

	state, err := s.getGameStateByID(ctx, id)
	if err != nil {
		return nil, "", err
	}
	return state, playerID, nil
}

// StartGame transitions a waiting game to 'playing'. Only the host may
// start it.
func (s *Store) StartGame(ctx context.Context, idOrCode, playerID string) (*models.GameState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	id, err := resolveGameID(ctx, tx, idOrCode)
	if err != nil {
		return nil, err
	}
	game, err := loadGame(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := expireIfNeeded(ctx, tx, game); err != nil {
		return nil, err
	}
	if game.HostID == nil || *game.HostID != playerID {
		return nil, ErrNotHost
	}
	if game.Status != "waiting" {
		return nil, ErrGameNotWaiting
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE games SET status = 'playing', started_at = ? WHERE id = ?
	`, now(), id); err != nil {
		return nil, fmt.Errorf("start game: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return s.getGameStateByID(ctx, id)
}

// SetCategory changes a waiting game's category. Only the host may do this,
// and only before the game has started.
func (s *Store) SetCategory(ctx context.Context, idOrCode, playerID, categoryID string) (*models.GameState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	id, err := resolveGameID(ctx, tx, idOrCode)
	if err != nil {
		return nil, err
	}
	game, err := loadGame(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := expireIfNeeded(ctx, tx, game); err != nil {
		return nil, err
	}
	if game.HostID == nil || *game.HostID != playerID {
		return nil, ErrNotHost
	}
	if game.Status != "waiting" {
		return nil, ErrGameNotWaiting
	}

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE id = ?`, categoryID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check category: %w", err)
	}
	if exists == 0 {
		return nil, ErrCategoryNotFound
	}

	if _, err := tx.ExecContext(ctx, `UPDATE games SET category_id = ? WHERE id = ?`, categoryID, id); err != nil {
		return nil, fmt.Errorf("set category: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM game_items WHERE game_id = ?`, id); err != nil {
		return nil, fmt.Errorf("clear game items: %w", err)
	}
	if err := selectGameItems(ctx, tx, id, categoryID); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return s.getGameStateByID(ctx, id)
}

// SetDuration changes a waiting game's round duration. Only the host may
// do this, and only before the game has started. The caller is
// responsible for range-checking durationSeconds against
// MinDurationSeconds/MaxDurationSeconds.
func (s *Store) SetDuration(ctx context.Context, idOrCode, playerID string, durationSeconds int) (*models.GameState, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	id, err := resolveGameID(ctx, tx, idOrCode)
	if err != nil {
		return nil, err
	}
	game, err := loadGame(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := expireIfNeeded(ctx, tx, game); err != nil {
		return nil, err
	}
	if game.HostID == nil || *game.HostID != playerID {
		return nil, ErrNotHost
	}
	if game.Status != "waiting" {
		return nil, ErrGameNotWaiting
	}

	if _, err := tx.ExecContext(ctx, `UPDATE games SET duration_seconds = ? WHERE id = ?`, durationSeconds, id); err != nil {
		return nil, fmt.Errorf("set duration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return s.getGameStateByID(ctx, id)
}

// RecordCapture records a player capturing an item. If the player has then
// captured every item in the category, the game finishes immediately
// (first to complete).
func (s *Store) RecordCapture(ctx context.Context, idOrCode, playerID, itemID string, confidence *float64) (*models.GameState, *models.Capture, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	id, err := resolveGameID(ctx, tx, idOrCode)
	if err != nil {
		return nil, nil, err
	}
	game, err := loadGame(ctx, tx, id)
	if err != nil {
		return nil, nil, err
	}
	if err := expireIfNeeded(ctx, tx, game); err != nil {
		return nil, nil, err
	}
	if game.Status != "playing" {
		return nil, nil, ErrGameNotPlaying
	}

	var playerGameID string
	if err := tx.QueryRowContext(ctx, `SELECT game_id FROM players WHERE id = ?`, playerID).Scan(&playerGameID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrPlayerNotFound
		}
		return nil, nil, fmt.Errorf("load player: %w", err)
	}
	if playerGameID != id {
		return nil, nil, ErrPlayerNotInGame
	}

	var inGame int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM game_items WHERE game_id = ? AND item_id = ?
	`, id, itemID).Scan(&inGame); err != nil {
		return nil, nil, fmt.Errorf("check item in game: %w", err)
	}
	if inGame == 0 {
		return nil, nil, ErrItemNotFound
	}

	var alreadyCaptured int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM captures WHERE game_id = ? AND player_id = ? AND item_id = ?
	`, id, playerID, itemID).Scan(&alreadyCaptured); err != nil {
		return nil, nil, fmt.Errorf("check existing capture: %w", err)
	}
	if alreadyCaptured > 0 {
		return nil, nil, ErrAlreadyCaptured
	}

	captureID := uuid.NewString()
	capturedAt := now()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO captures (id, game_id, player_id, item_id, confidence, captured_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, captureID, id, playerID, itemID, confidence, capturedAt); err != nil {
		return nil, nil, fmt.Errorf("insert capture: %w", err)
	}

	var totalItems, playerCaptures int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_items WHERE game_id = ?`, id).Scan(&totalItems); err != nil {
		return nil, nil, fmt.Errorf("count items: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM captures WHERE game_id = ? AND player_id = ?`, id, playerID).Scan(&playerCaptures); err != nil {
		return nil, nil, fmt.Errorf("count player captures: %w", err)
	}
	if totalItems > 0 && playerCaptures >= totalItems {
		if _, err := tx.ExecContext(ctx, `
			UPDATE games SET status = 'finished', ended_at = ? WHERE id = ? AND status = 'playing'
		`, capturedAt, id); err != nil {
			return nil, nil, fmt.Errorf("finish game: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}

	state, err := s.getGameStateByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	capture := &models.Capture{
		ID:         captureID,
		GameID:     id,
		PlayerID:   playerID,
		ItemID:     itemID,
		Confidence: confidence,
		CapturedAt: capturedAt,
	}
	return state, capture, nil
}

func (s *Store) getGameStateByID(ctx context.Context, id string) (*models.GameState, error) {
	game, err := loadGame(ctx, s.db, id)
	if err != nil {
		return nil, err
	}
	if err := expireIfNeeded(ctx, s.db, game); err != nil {
		return nil, err
	}
	items, err := loadItemsForGame(ctx, s.db, game.ID)
	if err != nil {
		return nil, err
	}
	players, err := loadPlayers(ctx, s.db, game.ID)
	if err != nil {
		return nil, err
	}
	return &models.GameState{Game: *game, Items: items, Players: players}, nil
}

func (s *Store) uniqueJoinCode(ctx context.Context) (string, error) {
	const maxAttempts = 10
	for range maxAttempts {
		code, err := randomJoinCode()
		if err != nil {
			return "", err
		}
		var exists int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM games WHERE join_code = ?`, code).Scan(&exists); err != nil {
			return "", fmt.Errorf("check join code: %w", err)
		}
		if exists == 0 {
			return code, nil
		}
	}
	return "", fmt.Errorf("could not generate a unique join code after %d attempts", maxAttempts)
}

func randomJoinCode() (string, error) {
	b := make([]byte, joinCodeLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(joinCodeAlphabet))))
		if err != nil {
			return "", fmt.Errorf("generate join code: %w", err)
		}
		b[i] = joinCodeAlphabet[n.Int64()]
	}
	return string(b), nil
}

// resolveGameID accepts either a game's internal ID or its public join code
// and returns the canonical ID, so every /api/games/:id route can be hit
// with whichever value the client currently has on hand.
func resolveGameID(ctx context.Context, q queryer, idOrCode string) (string, error) {
	var id string
	err := q.QueryRowContext(ctx, `SELECT id FROM games WHERE id = ?`, idOrCode).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("lookup game by id: %w", err)
	}

	err = q.QueryRowContext(ctx, `SELECT id FROM games WHERE join_code = ?`, strings.ToUpper(idOrCode)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrGameNotFound
	}
	if err != nil {
		return "", fmt.Errorf("lookup game by join code: %w", err)
	}
	return id, nil
}

func loadGame(ctx context.Context, q queryer, id string) (*models.Game, error) {
	var g models.Game
	var hostID, startedAt, endedAt sql.NullString
	err := q.QueryRowContext(ctx, `
		SELECT id, join_code, host_id, category_id, status, duration_seconds, created_at, started_at, ended_at
		FROM games WHERE id = ?
	`, id).Scan(&g.ID, &g.JoinCode, &hostID, &g.CategoryID, &g.Status, &g.DurationSeconds, &g.CreatedAt, &startedAt, &endedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrGameNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load game: %w", err)
	}
	g.HostID = nullStringPtr(hostID)
	g.StartedAt = nullStringPtr(startedAt)
	g.EndedAt = nullStringPtr(endedAt)
	return &g, nil
}

// expireIfNeeded lazily transitions a 'playing' game to 'finished' once its
// duration has elapsed, instead of running a background timer goroutine.
// It mutates g in place to reflect the new status if it changes.
func expireIfNeeded(ctx context.Context, q queryer, g *models.Game) error {
	if g.Status != "playing" || g.StartedAt == nil {
		return nil
	}

	started, err := time.Parse(TimeLayout, *g.StartedAt)
	if err != nil {
		return fmt.Errorf("parse started_at: %w", err)
	}
	deadline := started.Add(time.Duration(g.DurationSeconds) * time.Second)
	if time.Now().UTC().Before(deadline) {
		return nil
	}

	endedAt := deadline.Format(TimeLayout)
	res, err := q.ExecContext(ctx, `
		UPDATE games SET status = 'finished', ended_at = ? WHERE id = ? AND status = 'playing'
	`, endedAt, g.ID)
	if err != nil {
		return fmt.Errorf("expire game: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		g.Status = "finished"
		g.EndedAt = &endedAt
	}
	return nil
}

// selectGameItems draws up to TasksPerGame random items from categoryID's
// item pool and records them as gameID's tasks for this round. If the pool
// has fewer items than TasksPerGame, every item in it is used.
func selectGameItems(ctx context.Context, tx *sql.Tx, gameID, categoryID string) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM items WHERE category_id = ?`, categoryID)
	if err != nil {
		return fmt.Errorf("load item pool: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan item id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	mathrand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	if len(ids) > TasksPerGame {
		ids = ids[:TasksPerGame]
	}

	for _, itemID := range ids {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO game_items (game_id, item_id) VALUES (?, ?)
		`, gameID, itemID); err != nil {
			return fmt.Errorf("insert game item: %w", err)
		}
	}
	return nil
}

func loadItemsForGame(ctx context.Context, q queryer, gameID string) ([]models.Item, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT i.id, i.category_id, i.label, i.display_name
		FROM items i
		JOIN game_items gi ON gi.item_id = i.id
		WHERE gi.game_id = ?
		ORDER BY i.display_name
	`, gameID)
	if err != nil {
		return nil, fmt.Errorf("load items: %w", err)
	}
	defer rows.Close()

	items := []models.Item{}
	for rows.Next() {
		var it models.Item
		if err := rows.Scan(&it.ID, &it.CategoryID, &it.Label, &it.DisplayName); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func loadPlayers(ctx context.Context, q queryer, gameID string) ([]models.Player, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT p.id, p.game_id, p.name, p.is_host, p.connected_at, p.disconnected_at,
		       (SELECT COUNT(*) FROM captures c WHERE c.player_id = p.id) AS score
		FROM players p
		WHERE p.game_id = ?
		ORDER BY score DESC, p.connected_at ASC
	`, gameID)
	if err != nil {
		return nil, fmt.Errorf("load players: %w", err)
	}
	defer rows.Close()

	players := []models.Player{}
	for rows.Next() {
		var p models.Player
		var disconnectedAt sql.NullString
		if err := rows.Scan(&p.ID, &p.GameID, &p.Name, &p.IsHost, &p.ConnectedAt, &disconnectedAt, &p.Score); err != nil {
			return nil, fmt.Errorf("scan player: %w", err)
		}
		p.DisconnectedAt = nullStringPtr(disconnectedAt)
		p.CapturedItemIDs = []string{}
		players = append(players, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	captured, err := loadCapturedItemIDsByPlayer(ctx, q, gameID)
	if err != nil {
		return nil, err
	}
	for i := range players {
		if ids, ok := captured[players[i].ID]; ok {
			players[i].CapturedItemIDs = ids
		}
	}
	return players, nil
}

func loadCapturedItemIDsByPlayer(ctx context.Context, q queryer, gameID string) (map[string][]string, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT player_id, item_id FROM captures WHERE game_id = ?
	`, gameID)
	if err != nil {
		return nil, fmt.Errorf("load captures: %w", err)
	}
	defer rows.Close()

	byPlayer := make(map[string][]string)
	for rows.Next() {
		var playerID, itemID string
		if err := rows.Scan(&playerID, &itemID); err != nil {
			return nil, fmt.Errorf("scan capture: %w", err)
		}
		byPlayer[playerID] = append(byPlayer[playerID], itemID)
	}
	return byPlayer, rows.Err()
}

func nullStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}
