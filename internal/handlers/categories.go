package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/mogensen/lensrace/internal/catalog"
)

// Category is the API representation of a predefined item list.
type Category struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
}

// CategoryHandler serves the predefined category list.
type CategoryHandler struct {
	Catalog *catalog.Catalog
}

// List returns all categories, ordered by name.
func (h *CategoryHandler) List(c *fiber.Ctx) error {
	source := h.Catalog.Categories()
	categories := make([]Category, len(source))
	for i, cat := range source {
		categories[i] = Category{ID: cat.ID, Name: cat.Name, Description: cat.Description, Icon: cat.Icon}
	}
	return c.JSON(categories)
}
