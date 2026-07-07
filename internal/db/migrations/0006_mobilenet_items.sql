-- Items whose label is only recognizable by the secondary MobileNet
-- detector (see frontend/src/lib/detector.ts), not COCO-SSD's 80 classes.
-- These are genuine, literal objects — unlike the COCO-only stand-ins in
-- 0005 — now possible because MobileNet's ~1000-class ImageNet vocabulary
-- actually includes them.
INSERT INTO items (id, category_id, label, display_name) VALUES
    ('camp-tent', 'camp-life', 'mountain tent', 'Tent'),
    ('camp-sleeping-bag', 'camp-life', 'sleeping bag', 'Sleeping Bag'),
    ('camp-boot', 'camp-life', 'cowboy boot', 'Boot'),
    ('camp-hatchet', 'camp-life', 'hatchet', 'Hatchet'),
    ('camp-canoe', 'camp-life', 'canoe', 'Canoe'),
    ('camp-compass', 'camp-life', 'magnetic compass', 'Compass'),
    ('camp-torch', 'camp-life', 'torch', 'Torch'),
    ('forest-daisy', 'forest', 'daisy', 'Daisy'),
    ('forest-mushroom', 'forest', 'mushroom', 'Mushroom'),
    ('forest-acorn', 'forest', 'acorn', 'Acorn');
