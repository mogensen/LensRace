-- Seed categories and items. Labels match TensorFlow.js COCO-SSD class names
-- so on-device detections can be matched directly against item.label.

INSERT INTO categories (id, name, description) VALUES
    ('house-essentials', 'Around the House', 'Find everyday objects around your home.'),
    ('city-scavenger', 'Out and About', 'Find objects out in the city or neighborhood.');

INSERT INTO items (id, category_id, label, display_name) VALUES
    ('house-chair', 'house-essentials', 'chair', 'Chair'),
    ('house-cup', 'house-essentials', 'cup', 'Cup'),
    ('house-book', 'house-essentials', 'book', 'Book'),
    ('house-clock', 'house-essentials', 'clock', 'Clock'),
    ('house-laptop', 'house-essentials', 'laptop', 'Laptop'),
    ('house-bottle', 'house-essentials', 'bottle', 'Bottle'),
    ('city-car', 'city-scavenger', 'car', 'Car'),
    ('city-bicycle', 'city-scavenger', 'bicycle', 'Bicycle'),
    ('city-traffic-light', 'city-scavenger', 'traffic light', 'Traffic Light'),
    ('city-dog', 'city-scavenger', 'dog', 'Dog'),
    ('city-backpack', 'city-scavenger', 'backpack', 'Backpack'),
    ('city-bench', 'city-scavenger', 'bench', 'Bench');
