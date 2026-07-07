-- Games table: stores game sessions. category_id references a category
-- defined in internal/catalog's embedded YAML, not a database table —
-- categories/items are static application content, not persisted state,
-- so they aren't modeled here (see internal/catalog).
CREATE TABLE games (
    id               TEXT PRIMARY KEY,
    join_code        TEXT NOT NULL UNIQUE,
    host_id          TEXT,
    category_id      TEXT NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('waiting', 'countdown', 'playing', 'finished')),
    duration_seconds INTEGER NOT NULL DEFAULT 300,
    created_at       TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    started_at       TEXT,
    ended_at         TEXT,
    FOREIGN KEY (host_id) REFERENCES players(id)
);

-- Players table: stores player information
CREATE TABLE players (
    id              TEXT PRIMARY KEY,
    game_id         TEXT NOT NULL,
    name            TEXT NOT NULL,
    is_host         BOOLEAN NOT NULL DEFAULT FALSE,
    connected_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    disconnected_at TEXT,
    FOREIGN KEY (game_id) REFERENCES games(id)
);

-- Captures table: records a player successfully photographing an item.
-- item_id references an item defined in internal/catalog, not a database
-- table.
CREATE TABLE captures (
    id          TEXT PRIMARY KEY,
    game_id     TEXT NOT NULL,
    player_id   TEXT NOT NULL,
    item_id     TEXT NOT NULL,
    confidence  REAL,
    captured_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (game_id, player_id, item_id),
    FOREIGN KEY (game_id) REFERENCES games(id),
    FOREIGN KEY (player_id) REFERENCES players(id)
);

-- Game items table: the specific items randomly drawn from a category's
-- pool (internal/catalog) for a single game, so a game's task list stays
-- fixed once chosen even though the pool it was drawn from has more items
-- than a round uses. item_id references an item in internal/catalog, not
-- a database table.
CREATE TABLE game_items (
    game_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    PRIMARY KEY (game_id, item_id),
    FOREIGN KEY (game_id) REFERENCES games(id)
);

CREATE INDEX idx_players_game ON players(game_id);
CREATE INDEX idx_captures_game ON captures(game_id);
CREATE INDEX idx_game_items_game ON game_items(game_id);
