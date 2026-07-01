-- Expand each category's item pool beyond the number of tasks used in a
-- single game, so a game can draw a random subset instead of always using
-- every item. Labels match TensorFlow.js COCO-SSD class names.
INSERT INTO items (id, category_id, label, display_name) VALUES
    ('house-keyboard', 'house-essentials', 'keyboard', 'Keyboard'),
    ('house-remote', 'house-essentials', 'remote', 'Remote'),
    ('house-vase', 'house-essentials', 'vase', 'Vase'),
    ('house-scissors', 'house-essentials', 'scissors', 'Scissors'),
    ('house-teddy-bear', 'house-essentials', 'teddy bear', 'Teddy Bear'),
    ('house-potted-plant', 'house-essentials', 'potted plant', 'Potted Plant'),
    ('city-bus', 'city-scavenger', 'bus', 'Bus'),
    ('city-truck', 'city-scavenger', 'truck', 'Truck'),
    ('city-motorcycle', 'city-scavenger', 'motorcycle', 'Motorcycle'),
    ('city-stop-sign', 'city-scavenger', 'stop sign', 'Stop Sign'),
    ('city-fire-hydrant', 'city-scavenger', 'fire hydrant', 'Fire Hydrant'),
    ('city-umbrella', 'city-scavenger', 'umbrella', 'Umbrella');
