package handlers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/mogensen/lensrace/internal/models"
	"github.com/mogensen/lensrace/internal/store"
)

// GameHandler exposes the game lifecycle API: create, join, start, capture.
type GameHandler struct {
	Store *store.Store
}

type createGameRequest struct {
	CategoryID      string `json:"categoryId"`
	HostName        string `json:"hostName"`
	DurationSeconds int    `json:"durationSeconds"`
}

type joinGameRequest struct {
	Name string `json:"name"`
}

type startGameRequest struct {
	PlayerID string `json:"playerId"`
}

type captureRequest struct {
	PlayerID   string   `json:"playerId"`
	ItemID     string   `json:"itemId"`
	Confidence *float64 `json:"confidence"`
}

// sessionResponse wraps a game state with the ID of the player who just
// made the request. There is no auth/session layer yet, so the client is
// expected to hold onto playerId locally and send it back on subsequent
// start/capture calls.
type sessionResponse struct {
	*models.GameState
	PlayerID string `json:"playerId"`
}

type captureResponse struct {
	*models.GameState
	Capture models.Capture `json:"capture"`
}

// Create handles POST /api/games.
func (h *GameHandler) Create(c *fiber.Ctx) error {
	var req createGameRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	hostName, err := normalizeName(req.HostName)
	if err != nil {
		return err
	}
	if strings.TrimSpace(req.CategoryID) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "categoryId is required")
	}

	duration := req.DurationSeconds
	if duration == 0 {
		duration = store.DefaultDurationSeconds
	}
	if duration < store.MinDurationSeconds || duration > store.MaxDurationSeconds {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf(
			"durationSeconds must be between %d and %d", store.MinDurationSeconds, store.MaxDurationSeconds))
	}

	state, playerID, err := h.Store.CreateGame(c.Context(), req.CategoryID, hostName, duration)
	if err != nil {
		return mapStoreError(err)
	}

	return c.Status(fiber.StatusCreated).JSON(sessionResponse{GameState: state, PlayerID: playerID})
}

// Get handles GET /api/games/:id.
func (h *GameHandler) Get(c *fiber.Ctx) error {
	state, err := h.Store.GetGameState(c.Context(), c.Params("id"))
	if err != nil {
		return mapStoreError(err)
	}
	return c.JSON(state)
}

// Join handles POST /api/games/:id/join.
func (h *GameHandler) Join(c *fiber.Ctx) error {
	var req joinGameRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	name, err := normalizeName(req.Name)
	if err != nil {
		return err
	}

	state, playerID, err := h.Store.JoinGame(c.Context(), c.Params("id"), name)
	if err != nil {
		return mapStoreError(err)
	}

	return c.Status(fiber.StatusCreated).JSON(sessionResponse{GameState: state, PlayerID: playerID})
}

// Start handles POST /api/games/:id/start.
func (h *GameHandler) Start(c *fiber.Ctx) error {
	var req startGameRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.PlayerID) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "playerId is required")
	}

	state, err := h.Store.StartGame(c.Context(), c.Params("id"), req.PlayerID)
	if err != nil {
		return mapStoreError(err)
	}

	return c.JSON(state)
}

// Capture handles POST /api/games/:id/captures.
func (h *GameHandler) Capture(c *fiber.Ctx) error {
	var req captureRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if strings.TrimSpace(req.PlayerID) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "playerId is required")
	}
	if strings.TrimSpace(req.ItemID) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "itemId is required")
	}
	if req.Confidence != nil && (*req.Confidence < 0 || *req.Confidence > 1) {
		return fiber.NewError(fiber.StatusBadRequest, "confidence must be between 0 and 1")
	}

	state, capture, err := h.Store.RecordCapture(c.Context(), c.Params("id"), req.PlayerID, req.ItemID, req.Confidence)
	if err != nil {
		return mapStoreError(err)
	}

	return c.Status(fiber.StatusCreated).JSON(captureResponse{GameState: state, Capture: *capture})
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fiber.NewError(fiber.StatusBadRequest, "name is required")
	}
	if len(name) > 30 {
		return "", fiber.NewError(fiber.StatusBadRequest, "name must be 30 characters or fewer")
	}
	return name, nil
}

func mapStoreError(err error) error {
	switch {
	case errors.Is(err, store.ErrCategoryNotFound),
		errors.Is(err, store.ErrGameNotFound),
		errors.Is(err, store.ErrPlayerNotFound),
		errors.Is(err, store.ErrItemNotFound):
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	case errors.Is(err, store.ErrNotHost),
		errors.Is(err, store.ErrPlayerNotInGame):
		return fiber.NewError(fiber.StatusForbidden, err.Error())
	case errors.Is(err, store.ErrGameNotWaiting),
		errors.Is(err, store.ErrGameNotPlaying),
		errors.Is(err, store.ErrAlreadyCaptured):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	default:
		return fiber.NewError(fiber.StatusInternalServerError, "internal server error")
	}
}
