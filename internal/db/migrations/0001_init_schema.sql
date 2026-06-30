-- Categories table: predefined item lists (seeded, read-only at runtime)
CREATE TABLE categories (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT
);

-- Items table: the objects belonging to a category
CREATE TABLE items (
    id           TEXT PRIMARY KEY,
    category_id  TEXT NOT NULL,
    label        TEXT NOT NULL,
    display_name TEXT NOT NULL,
    FOREIGN KEY (category_id) REFERENCES categories(id)
);

-- Games table: stores game sessions
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
    FOREIGN KEY (host_id) REFERENCES players(id),
    FOREIGN KEY (category_id) REFERENCES categories(id)
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

-- Captures table: records a player successfully photographing an item
CREATE TABLE captures (
    id          TEXT PRIMARY KEY,
    game_id     TEXT NOT NULL,
    player_id   TEXT NOT NULL,
    item_id     TEXT NOT NULL,
    confidence  REAL,
    captured_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (game_id, player_id, item_id),
    FOREIGN KEY (game_id) REFERENCES games(id),
    FOREIGN KEY (player_id) REFERENCES players(id),
    FOREIGN KEY (item_id) REFERENCES items(id)
);

CREATE INDEX idx_players_game ON players(game_id);
CREATE INDEX idx_captures_game ON captures(game_id);
