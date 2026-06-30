// Package server wires up the Fiber app: middleware and route registration.
package server

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/mogensen/lensrace/internal/handlers"
)

// New builds a Fiber app with middleware and API routes registered.
func New(conn *sql.DB) *fiber.App {
	app := fiber.New()

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New())

	categories := &handlers.CategoryHandler{DB: conn}

	api := app.Group("/api")
	api.Get("/health", handlers.Health)
	api.Get("/categories", categories.List)

	return app
}
