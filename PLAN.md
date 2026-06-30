# 🎯 LensRace Development Plan

> **Multiplayer mobile web game** where players race to photograph objects from a shared list using on-device image recognition.
> Built with a **Go Fiber + SQLite** backend and a **Vue.js PWA** frontend.

---

## ✨ Features & Requirements

### Core Gameplay
- [x] Create a new game as host
- [x] Join an existing game with a code
- [x] Select game category (predefined lists)
- [x] Start game (host only)
- [x] View item list during gameplay
- [ ] Camera access for photographing items
- [ ] Automatic item detection via image recognition
- [x] Score tracking per player
- [ ] Countdown timer (configurable duration) — duration is configurable and enforced server-side; client-facing countdown UI is still pending
- [x] Game end detection (first to complete or time expires)
- [ ] Results screen with final rankings

### Technical Features
- [ ] **PWA**: Installable, offline-capable
- [ ] **Offline Mode**: Play without internet, sync when reconnected
- [x] **Real-time Leaderboard**: Live updates during gameplay
- [ ] **On-device Image Recognition**: No server processing, privacy-first
- [ ] **Responsive Design**: Mobile-first, works on all screen sizes
- [ ] **Accessibility**: Screen reader support, high contrast mode

### Backend Features
- [x] RESTful API with Go Fiber
- [x] In-memory caching for live game state (persisted to SQLite)
- [x] Game session management with unique join codes
- [ ] Player connection tracking — `connected_at`/`disconnected_at` exist in schema, disconnect detection not wired up yet
- [x] Real-time updates via Server-Sent Events (SSE)
- [ ] Rate limiting and abuse prevention

---

## 🛠️ Technical Stack

### Backend
| Component | Technology               |
| --------- | ------------------------ |
| Runtime   | Go + Fiber               |
| Database  | SQLite                   |
| API       | RESTful HTTP endpoints   |
| Real-time | Server-Sent Events (SSE) |
| Testing   | Go `testing` package     |

### Frontend
| Component         | Technology                                |
| ----------------- | ----------------------------------------- |
| Framework         | Vue.js                                    |
| Styling           | Tailwind CSS                              |
| Image Recognition | TensorFlow.js + COCO-SSD                  |
| Camera            | MediaDevices API                          |
| Offline           | Service Worker + LocalStorage + IndexedDB |
| Testing           | Playwright                                |

### DevOps
| Component       | Technology              |
| --------------- | ----------------------- |
| Package Manager | pnpm (frontend)         |
| Bundler         | Vite                    |
| Deployment      | Docker container        |
| CI/CD           | GitHub Actions          |
| Linting         | ESLint + Prettier (FE), `gofmt` + `go vet` (BE) |
| Type Checking   | TypeScript (frontend)   |

---

## 🗃️ Data Model (SQLite)

> Live game state is cached in-memory for fast SSE fan-out and written through to
> SQLite so sessions survive a restart. Per-player **score is derived** as the count
> of that player's rows in `captures`.

### Database Schema

```sql
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
    label        TEXT NOT NULL, -- COCO-SSD class label to match detections against
    display_name TEXT NOT NULL,
    FOREIGN KEY (category_id) REFERENCES categories(id)
);

-- Games table: stores game sessions
CREATE TABLE games (
    id               TEXT PRIMARY KEY,
    join_code        TEXT NOT NULL UNIQUE,
    host_id          TEXT,            -- nullable to break the circular FK on insert
    category_id      TEXT NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('waiting', 'countdown', 'playing', 'finished')),
    duration_seconds INTEGER NOT NULL DEFAULT 300, -- 5 minutes default
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
    UNIQUE (game_id, player_id, item_id), -- a player can only score each item once
    FOREIGN KEY (game_id) REFERENCES games(id),
    FOREIGN KEY (player_id) REFERENCES players(id),
    FOREIGN KEY (item_id) REFERENCES items(id)
);

CREATE INDEX idx_players_game ON players(game_id);
CREATE INDEX idx_captures_game ON captures(game_id);
```

> **Note on the circular reference:** `games.host_id → players.id` and
> `players.game_id → games.id` reference each other. Insert the game first with
> `host_id = NULL`, create the host player, then `UPDATE games.host_id`.

---

## 🔌 API Surface (Go Fiber)

| Method | Path                          | Description                                  |
| ------ | ----------------------------- | -------------------------------------------- |
| GET    | `/api/categories`             | List predefined categories                   |
| POST   | `/api/games`                  | Create a game (returns id + join code)       |
| GET    | `/api/games/:id`              | Game state (players, status, items)          |
| POST   | `/api/games/:id/join`         | Join a game with a name                      |
| POST   | `/api/games/:id/start`        | Start the game (host only)                   |
| POST   | `/api/games/:id/captures`     | Record a detected item for the player        |
| GET    | `/api/games/:id/events`       | **SSE** stream: live leaderboard + status    |

---

## 🧭 Milestones

1. ✅ **Backend skeleton** — Fiber server, SQLite migrations, seed categories/items, health check.
2. ✅ **Game lifecycle API** — create/join/start, player tracking, derived scoring, lazy time-based + first-to-complete game end detection.
3. ✅ **Real-time** — SSE leaderboard + status broadcast, in-memory cache, background expiry watcher.
4. **Frontend skeleton** — Vue + Vite + Tailwind, routing, lobby/join screens.
5. **Camera + recognition** — MediaDevices capture, TensorFlow.js COCO-SSD, match to items.
6. **Gameplay loop** — timer, capture submission, live leaderboard, results screen.
7. **PWA + offline** — service worker, installability, offline play with reconnect sync.
8. **Polish** — accessibility, rate limiting, CI (GitHub Actions), Docker deploy.
