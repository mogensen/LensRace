package catalog

import "testing"

// cocoSsdClasses is the fixed, closed set of object classes the frontend's
// primary on-device detector (TensorFlow.js COCO-SSD) can ever recognize —
// see coco-ssd's classes.ts. Every item's label must be in this set or in
// imagenetTokens (the secondary MobileNet model's vocabulary, generated in
// imagenet_classes_test.go), or it can never be auto-captured.
var cocoSsdClasses = map[string]bool{
	"person": true, "bicycle": true, "car": true, "motorcycle": true, "airplane": true,
	"bus": true, "train": true, "truck": true, "boat": true, "traffic light": true,
	"fire hydrant": true, "stop sign": true, "parking meter": true, "bench": true, "bird": true,
	"cat": true, "dog": true, "horse": true, "sheep": true, "cow": true,
	"elephant": true, "bear": true, "zebra": true, "giraffe": true, "backpack": true,
	"umbrella": true, "handbag": true, "tie": true, "suitcase": true, "frisbee": true,
	"skis": true, "snowboard": true, "sports ball": true, "kite": true, "baseball bat": true,
	"baseball glove": true, "skateboard": true, "surfboard": true, "tennis racket": true, "bottle": true,
	"wine glass": true, "cup": true, "fork": true, "knife": true, "spoon": true,
	"bowl": true, "banana": true, "apple": true, "sandwich": true, "orange": true,
	"broccoli": true, "carrot": true, "hot dog": true, "pizza": true, "donut": true,
	"cake": true, "chair": true, "couch": true, "potted plant": true, "bed": true,
	"dining table": true, "toilet": true, "tv": true, "laptop": true, "mouse": true,
	"remote": true, "keyboard": true, "cell phone": true, "microwave": true, "oven": true,
	"toaster": true, "sink": true, "refrigerator": true, "book": true, "clock": true,
	"vase": true, "scissors": true, "teddy bear": true, "hair drier": true, "toothbrush": true,
}

// isDetectableLabel reports whether label is something at least one of the
// frontend's two on-device models (COCO-SSD or MobileNet) can recognize —
// see frontend/src/lib/detector.ts, which picks between them per-item using
// the exact same two sets.
func isDetectableLabel(label string) bool {
	return cocoSsdClasses[label] || imagenetTokens[label]
}

func mustLoad(t *testing.T) *Catalog {
	t.Helper()
	cat, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cat
}

func TestLoadParsesEmbeddedCatalog(t *testing.T) {
	cat := mustLoad(t)
	if len(cat.categories) == 0 {
		t.Fatal("expected at least one category")
	}
}

func TestCategoriesAreSortedByName(t *testing.T) {
	cat := mustLoad(t)
	categories := cat.Categories()
	for i := 1; i < len(categories); i++ {
		if categories[i-1].Name > categories[i].Name {
			t.Fatalf("categories not sorted by name: %q came before %q", categories[i-1].Name, categories[i].Name)
		}
	}
}

func TestCategoryExists(t *testing.T) {
	cat := mustLoad(t)
	if !cat.CategoryExists("house-essentials") {
		t.Error("expected house-essentials to exist")
	}
	if cat.CategoryExists("does-not-exist") {
		t.Error("expected does-not-exist to not exist")
	}
}

func TestItemLookupReturnsOwningCategory(t *testing.T) {
	cat := mustLoad(t)
	item, categoryID, ok := cat.Item("house-chair")
	if !ok {
		t.Fatal("expected house-chair to be found")
	}
	if categoryID != "house-essentials" {
		t.Errorf("categoryID = %q, want house-essentials", categoryID)
	}
	if item.Label != "chair" {
		t.Errorf("label = %q, want chair", item.Label)
	}

	if _, _, ok := cat.Item("does-not-exist"); ok {
		t.Error("expected does-not-exist to not be found")
	}
}

func TestItemIDsInCategoryMatchesItemCount(t *testing.T) {
	cat := mustLoad(t)
	ids := cat.ItemIDsInCategory("house-essentials")
	if len(ids) != 12 {
		t.Errorf("len(ids) = %d, want 12", len(ids))
	}
}

func TestItemLabelsAreDetectableByAModel(t *testing.T) {
	cat := mustLoad(t)

	var undetectable []string
	for _, c := range cat.categories {
		for _, it := range c.Items {
			if !isDetectableLabel(it.Label) {
				undetectable = append(undetectable, it.ID+" (label="+it.Label+")")
			}
		}
	}
	if len(undetectable) > 0 {
		t.Fatalf("items with a label neither on-device model can ever match: %v", undetectable)
	}
}

func TestForestAndCampLifeHaveAtLeast30Items(t *testing.T) {
	cat := mustLoad(t)

	for _, id := range []string{"forest", "camp-life"} {
		items := cat.ItemIDsInCategory(id)
		if len(items) < 30 {
			t.Errorf("category %s has %d items, want at least 30", id, len(items))
		}
	}
}
