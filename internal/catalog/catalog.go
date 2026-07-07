// Package catalog holds the static, embedded set of categories and items
// players can hunt for — see catalog.yaml for the data itself and the
// ground rules for what belongs in it. Unlike games/players/captures
// (dynamic state that lives in SQLite, see internal/db), this content
// never changes at runtime, so it's loaded once from an embedded file
// instead of being seeded into the database.
package catalog

import (
	"embed"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

//go:embed catalog.yaml
var catalogFS embed.FS

// Item is an object belonging to a category that players try to capture.
type Item struct {
	ID          string `yaml:"id"`
	Label       string `yaml:"label"`
	DisplayName string `yaml:"displayName"`
}

// Category is a predefined, themed list of items.
type Category struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Icon        string `yaml:"icon"`
	Items       []Item `yaml:"items"`
}

type document struct {
	Categories []Category `yaml:"categories"`
}

// Catalog is the parsed, indexed contents of catalog.yaml.
type Catalog struct {
	categories   []Category
	categoryByID map[string]*Category
	itemIndex    map[string]itemLookup
}

type itemLookup struct {
	Item
	CategoryID string
}

// Load parses the embedded catalog.yaml and validates it structurally:
// every category and item id must be unique, and every category/item must
// have a non-empty name/label/displayName. It does not validate that item
// labels are recognizable by an on-device detector — see catalog_test.go
// for that (a content-authoring safeguard, not a startup invariant).
func Load() (*Catalog, error) {
	raw, err := catalogFS.ReadFile("catalog.yaml")
	if err != nil {
		return nil, fmt.Errorf("read catalog.yaml: %w", err)
	}

	var doc document
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse catalog.yaml: %w", err)
	}

	cat := &Catalog{
		categories:   doc.Categories,
		categoryByID: make(map[string]*Category, len(doc.Categories)),
		itemIndex:    make(map[string]itemLookup),
	}

	for i := range cat.categories {
		c := &cat.categories[i]
		if c.ID == "" || c.Name == "" || c.Icon == "" {
			return nil, fmt.Errorf("category %d: id, name, and icon are required", i)
		}
		if _, dup := cat.categoryByID[c.ID]; dup {
			return nil, fmt.Errorf("duplicate category id %q", c.ID)
		}
		cat.categoryByID[c.ID] = c

		for _, it := range c.Items {
			if it.ID == "" || it.Label == "" || it.DisplayName == "" {
				return nil, fmt.Errorf("category %q: item missing id/label/displayName: %+v", c.ID, it)
			}
			if _, dup := cat.itemIndex[it.ID]; dup {
				return nil, fmt.Errorf("duplicate item id %q", it.ID)
			}
			cat.itemIndex[it.ID] = itemLookup{Item: it, CategoryID: c.ID}
		}
	}

	return cat, nil
}

// Categories returns every category, sorted by name (matches the ordering
// the /api/categories endpoint has always returned).
func (c *Catalog) Categories() []Category {
	sorted := make([]Category, len(c.categories))
	copy(sorted, c.categories)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	return sorted
}

// CategoryExists reports whether id is a known category.
func (c *Catalog) CategoryExists(id string) bool {
	_, ok := c.categoryByID[id]
	return ok
}

// ItemIDsInCategory returns the ids of every item in categoryID's pool, in
// no particular order (callers needing a specific order should sort).
func (c *Catalog) ItemIDsInCategory(categoryID string) []string {
	cat, ok := c.categoryByID[categoryID]
	if !ok {
		return nil
	}
	ids := make([]string, len(cat.Items))
	for i, it := range cat.Items {
		ids[i] = it.ID
	}
	return ids
}

// Item looks up an item by id, returning it along with the id of the
// category it belongs to.
func (c *Catalog) Item(id string) (item Item, categoryID string, ok bool) {
	lookup, found := c.itemIndex[id]
	if !found {
		return Item{}, "", false
	}
	return lookup.Item, lookup.CategoryID, true
}
