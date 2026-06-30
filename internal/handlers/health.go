package handlers

import "github.com/gofiber/fiber/v2"

// Health reports liveness for monitoring and the dev workflow.
func Health(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}
