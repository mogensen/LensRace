// Package server wires up the Fiber app: middleware and route registration.
package server

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/mogensen/lensrace/internal/handlers"
	"github.com/mogensen/lensrace/internal/realtime"
	"github.com/mogensen/lensrace/internal/store"
)

// New builds a Fiber app with middleware and API routes registered. hub
// caches live game state and fans out updates to SSE subscribers; the
// caller is responsible for keeping it fed (see realtime.WatchExpirations).
func New(conn *sql.DB, hub *realtime.Hub) *fiber.App {
	app := fiber.New()

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New())

	categories := &handlers.CategoryHandler{DB: conn}
	games := &handlers.GameHandler{Store: store.New(conn), Hub: hub}

	api := app.Group("/api")
	api.Get("/health", handlers.Health)
	api.Get("/categories", categories.List)

	api.Post("/games", games.Create)
	api.Get("/games/:id", games.Get)
	api.Post("/games/:id/join", games.Join)
	api.Post("/games/:id/start", games.Start)
	api.Post("/games/:id/captures", games.Capture)
	api.Get("/games/:id/events", games.Events)

	return app
}
