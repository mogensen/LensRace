// Package server wires up the Fiber app: middleware and route registration.
package server

import (
	"database/sql"
	"errors"
	"io/fs"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	frontenddist "github.com/mogensen/lensrace/frontend"
	"github.com/mogensen/lensrace/internal/handlers"
	"github.com/mogensen/lensrace/internal/realtime"
	"github.com/mogensen/lensrace/internal/store"
)

// New builds a Fiber app with middleware and API routes registered. hub
// caches live game state and fans out updates to SSE subscribers; the
// caller is responsible for keeping it fed (see realtime.WatchExpirations).
func New(conn *sql.DB, hub *realtime.Hub) *fiber.App {
	app := fiber.New(fiber.Config{
		// The default error handler returns plain text; the frontend needs
		// JSON error bodies to show user-facing messages.
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			var fiberErr *fiber.Error
			if errors.As(err, &fiberErr) {
				code = fiberErr.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

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
	api.Patch("/games/:id/category", games.SetCategory)
	api.Post("/games/:id/start", games.Start)
	api.Post("/games/:id/captures", games.Capture)
	api.Get("/games/:id/events", games.Events)

	registerFrontend(app)

	return app
}

// registerFrontend serves the built Vue SPA from the embedded
// frontend/dist, if the binary was built with `-tags embed_frontend` (see
// frontend/embed.go and the Makefile's `build` target). It's a no-op
// otherwise — the normal case for local development, where Vite serves the
// frontend on its own port instead. Registered after the API routes so it
// never intercepts /api/* requests: Fiber matches in registration order,
// and the API handlers above don't call Next().
func registerFrontend(app *fiber.App) {
	entries, err := frontenddist.DistFS.ReadDir(frontenddist.DistDir)
	if err != nil || len(entries) == 0 {
		return
	}

	sub, err := fs.Sub(frontenddist.DistFS, frontenddist.DistDir)
	if err != nil {
		return
	}

	app.Use(filesystem.New(filesystem.Config{
		Root:         http.FS(sub),
		Index:        "index.html",
		NotFoundFile: "index.html", // SPA fallback so client-side routes survive a refresh
	}))
}
