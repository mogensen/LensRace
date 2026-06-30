package handlers

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
)

// Category is the API representation of a predefined item list.
type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CategoryHandler serves the predefined category list.
type CategoryHandler struct {
	DB *sql.DB
}

// List returns all categories, ordered by name.
func (h *CategoryHandler) List(c *fiber.Ctx) error {
	rows, err := h.DB.Query(`SELECT id, name, COALESCE(description, '') FROM categories ORDER BY name`)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list categories")
	}
	defer rows.Close()

	categories := []Category{}
	for rows.Next() {
		var cat Category
		if err := rows.Scan(&cat.ID, &cat.Name, &cat.Description); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "failed to read category")
		}
		categories = append(categories, cat)
	}
	if err := rows.Err(); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to read categories")
	}

	return c.JSON(categories)
}
