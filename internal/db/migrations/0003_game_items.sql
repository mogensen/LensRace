-- Game items table: the specific items randomly drawn from a category's
-- pool for a single game, so a game's task list stays fixed once chosen
-- even though the pool it was drawn from has more items than a round uses.
CREATE TABLE game_items (
    game_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    PRIMARY KEY (game_id, item_id),
    FOREIGN KEY (game_id) REFERENCES games(id),
    FOREIGN KEY (item_id) REFERENCES items(id)
);

CREATE INDEX idx_game_items_game ON game_items(game_id);
