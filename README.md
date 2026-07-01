# 🎯 LensRace

Multiplayer mobile web game where players race to photograph objects from a shared
list using on-device image recognition. **Go Fiber + SQLite** backend, **Vue.js PWA**
frontend.

See [`PLAN.md`](./PLAN.md) for the full feature set, data model, and roadmap.

> **Status:** core gameplay loop works end-to-end — create/join, lobby,
> live SSE leaderboard, real on-device camera capture (TensorFlow.js
> COCO-SSD), results. See `PLAN.md` for what's done and what's left
> (PWA/offline, accessibility, polish).

---

## 📦 Prerequisites

| Tool        | Version       | Notes                                  |
| ----------- | ------------- | -------------------------------------- |
| Go          | 1.22+         | Backend runtime                        |
| Fiber CLI   | latest        | Live-reload dev server (see below)     |
| Node.js     | 20+           | Frontend toolchain                     |
| pnpm        | 9+            | Frontend package manager               |
| Docker      | optional      | Production image / deployment          |

Install the Fiber CLI (used for backend development):

```sh
go install github.com/gofiber/cli/fiber@latest
```

Verify it's on your `PATH`:

```sh
fiber version
```

---

## 🛠️ Makefile shortcuts

`make help` lists everything; the short version:

| Command               | Does                                                                |
| ---------------------- | -------------------------------------------------------------------|
| `make install`          | `go mod download` + `pnpm install`                                 |
| `make dev`              | Runs backend (`fiber dev`, or `go run .` if the CLI isn't installed) and frontend (`pnpm dev`) together; Ctrl+C stops both |
| `make stop`             | Kills anything listening on the backend/frontend ports (`:3000`/`:5173`) |
| `make clean`            | Removes `bin/`, `frontend/dist`, `frontend/public/vendor`, caches, and the dev SQLite database |
| `make build`            | Builds the frontend, then a single backend binary (`bin/server`) that serves the frontend **and** the API on one port |
| `make run`              | `make build`, then runs `bin/server`                                |
| `make public`           | `make build` + runs it + opens a public tunnel to it (ngrok → cloudflared → `npx localtunnel`, whichever is available) |
| `make test`             | `make test-backend test-frontend`                                  |
| `make test-backend`     | `go test ./... -race`                                              |
| `make test-frontend`    | Playwright e2e tests (starts the backend + frontend itself, see `playwright.config.ts`) |
| `make lint` / `make fmt`| Lint / format both backend and frontend                            |

`BACKEND_PORT`, `FRONTEND_PORT`, and `DB_PATH` are overridable, e.g. `make dev DB_PATH=/tmp/scratch.db`.

`make build`/`make run`/`make public` produce a genuinely single-process
deployment: the Go binary is compiled with `-tags embed_frontend`, which
embeds `frontend/dist` via `//go:embed` (see `frontend/embed.go`) and Fiber
serves it directly (`internal/server/server.go`), with an SPA fallback so
client-side routes survive a refresh. Without that tag (the default —
what `go build .` / `go run .` do), the binary has no embedded frontend at
all and only serves `/api/*`, which is what you want for local dev since
Vite serves the frontend on its own port instead.

---

## 🗂️ Repository Layout

```
LensRace/
├── main.go                    # entrypoint: config -> db -> migrate -> serve
├── internal/
│   ├── config/                # env-based configuration
│   ├── db/
│   │   ├── db.go               # SQLite connection + migration runner
│   │   └── migrations/         # embedded SQL schema + seed data
│   ├── handlers/               # HTTP handlers
│   └── server/                 # Fiber app + route registration
├── go.mod / go.sum
├── frontend/                  # Vue.js PWA (Vite + Tailwind)
├── PLAN.md                    # development plan & data model
└── README.md
```

---

## 🔧 Backend (Go Fiber)

All backend commands run from the **repo root**.

### Develop with live reload

The Fiber CLI watches your files and restarts the server on every change:

```sh
fiber dev
```

Useful flags for this project:

```sh
# Watch Go + template files, ignore build/vendor dirs.
fiber dev \
  -e go,tmpl,html \
  -D tmp,vendor,node_modules,dist,frontend
```

| Flag        | Purpose                                              |
| ----------- | ---------------------------------------------------- |
| `-e`        | Extra file extensions to watch                       |
| `-D`        | Directories to ignore                                |
| `-d`        | Restart debounce delay (e.g. `-d 2s`)                |
| `--pre-run` | Command(s) to run before each restart                |
| `-a`        | Arguments passed to the app (e.g. `-a "-port=8080"`) |
| `-t`        | Build/run target package                             |

### Build & run without the CLI

```sh
go build -o ./bin/server .
./bin/server
```

### Test

```sh
go test ./...
```

The API listens on `:3000` by default and is available at
<http://localhost:3000/api>. On startup it opens (or creates) the SQLite
database and applies any pending migrations automatically — no separate
migrate step is needed.

Available endpoints so far:

| Method | Path                       | Description                                         |
| ------ | -------------------------- | ---------------------------------------------------- |
| GET    | `/api/health`               | Liveness check                                       |
| GET    | `/api/categories`           | List predefined categories                           |
| POST   | `/api/games`                 | Create a game (body: `categoryId`, `hostName`, optional `durationSeconds`) |
| GET    | `/api/games/:id`             | Game state — `:id` accepts either the internal ID or the public join code |
| POST   | `/api/games/:id/join`        | Join a waiting game (body: `name`)                   |
| PATCH  | `/api/games/:id/category`    | Change category while waiting (body: `playerId`, `categoryId`; host only) |
| PATCH  | `/api/games/:id/duration`    | Change round length while waiting (body: `playerId`, `durationSeconds`; host only) |
| POST   | `/api/games/:id/start`       | Start the game (body: `playerId`; host only)         |
| POST   | `/api/games/:id/captures`    | Record a captured item (body: `playerId`, `itemId`, optional `confidence`) |
| GET    | `/api/games/:id/events`      | **SSE** stream of the full game state on every change (status + leaderboard) |

There's no auth/session layer yet — `playerId` is handed back in the create/join
response and the client is expected to hold onto it for `start`/`captures` calls.
A game auto-finishes when its `durationSeconds` elapses or when a player
captures every item in the game's task list — a random subset of the
category's item pool, drawn once when the game is created (or when the host
changes its category). Expiry is detected proactively by a
background watcher (checks an in-memory cache every second, so SSE clients see
the `finished` status pushed to them — no need to poll), not just lazily on the
next request.

The `/events` stream sends an `event: state` message with the full
`GameState` JSON as soon as you connect, then again on every join/start/capture/expiry,
plus a `:` comment line every 25s as a keepalive. There's no event-type
differentiation — decode every `data:` payload as a `GameState`.

### Configuration

| Env var   | Default        | Description                          |
| --------- | -------------- | ------------------------------------- |
| `PORT`    | `3000`         | HTTP listen port                      |
| `DB_PATH` | `lensrace.db`  | SQLite file path (`:memory:` for tests)|

---

## 🎨 Frontend (Vue.js PWA)

All frontend commands run from the `frontend/` directory.

```sh
cd frontend
pnpm install      # install dependencies
pnpm dev          # Vite dev server with HMR
pnpm build        # type-check + production build
pnpm preview      # serve the production build locally
pnpm test:e2e     # Playwright end-to-end tests (run `pnpm exec playwright install` first)
pnpm lint         # oxlint + ESLint (--fix)
pnpm format       # Prettier
```

The dev server proxies `/api` to the backend on `:3000` (configured in
`vite.config.ts`).

`pnpm test:e2e` is fully self-contained: `playwright.config.ts` builds and
starts the real Go backend (against a dedicated file at
`/tmp/lensrace-playwright-e2e.db`, not `:memory:` — see the comment there
for why) alongside the Vite dev server, so you don't need to start either
by hand first. `e2e/game.spec.ts` covers game creation and joining:
generated join codes, live SSE-driven player-list updates across two
browser contexts (host + guest), lowercase-code joins, and the validation/
error paths (missing name, missing code, unknown code, already-started
game).

Scaffolded with `create-vue` (TypeScript, Vue Router, ESLint+Prettier,
Playwright) plus Tailwind CSS v4 via `@tailwindcss/vite`. The UI implements
the "Snap Hunt" design end-to-end against the real backend — no mock data —
using the app's own game state and lobby/timer/leaderboard, not the design
prototype's simulated bot opponents.

| Route               | View          | Purpose                                                  |
| -------------------- | -------------- | ---------------------------------------------------------|
| `/`                   | `HomeView`     | Create a game or join one with a code                    |
| `/games/:id/lobby`    | `LobbyView`    | Share the join code, host picks category + round length, players + start|
| `/games/:id/play`     | `PlayView`     | Live timer, progress, item list, leaderboard, SNAP button|
| `/games/:id/results`  | `ResultsView`  | Podium, full ranking, confetti, play again                |

Supporting structure:

- `src/lib/api.ts` — typed fetch client for the backend (mirrors `internal/models`), plus an SSE subscription helper.
- `src/stores/game.ts` — a small reactive singleton (not Pinia — unnecessary for this scope) holding the live `GameState`, the current player's id, and the SSE connection; persists `{ gameId, playerId }` to `localStorage` so a page refresh mid-game doesn't lose your identity.
- `src/components/CameraOverlay.vue` — the aim/scan/done capture UI: a real `getUserMedia` feed with TensorFlow.js COCO-SSD running on-device against it every ~400ms, auto-triggering a capture on a sustained match. Tapping the shutter runs the same detection check once against the current frame (not a blind bypass) — a single match is enough there since it's a deliberate one-off action, vs. the passive loop's requirement of consecutive matches to debounce transient false positives.
- `src/lib/detector.ts` — loads TensorFlow.js/COCO-SSD. **Not** an ES import: it injects `<script>` tags pointing at `public/vendor/{tf,coco-ssd}.min.js` (copied from `node_modules` by `scripts/copy-vendor.mjs`, wired into `postinstall`/`predev`/`prebuild-only`). Reason: `tfjs-converter` has a class method literally named `import` (`async import(keys, values) {}`), and Vite's lightweight import-scanner misreads `import(` there as a dynamic-import call, corrupting the file — this happens in Vite's transform pipeline itself, so `optimizeDeps` include/exclude doesn't help. Loading via `<script>` sidesteps Vite's JS transform for these files entirely, and as a bonus keeps the ~1.4MB payload out of the initial bundle (`CameraOverlay` is lazy-loaded and only injects the scripts when the camera actually opens).
- `src/lib/{avatar,itemIcons,categoryIcons}.ts` — small client-side cosmetic lookups (emoji/color) for players/items/categories, since the backend doesn't model those.

Two backend additions came out of implementing this design: `PATCH /api/games/:id/category` and `PATCH /api/games/:id/duration` let the host change category and round length from the lobby (both are set *after* creating the game, not at creation time) — both broadcast over SSE, so every player's lobby updates live, not just the host's.

---

## 🚀 Running the full stack locally

Fastest path:

```sh
make dev
```

Or manually, in two terminals:

```sh
# Terminal 1 — backend with live reload (repo root)
fiber dev

# Terminal 2 — frontend with HMR
cd frontend && pnpm dev
```

Then open the URL printed by Vite (typically <http://localhost:5173>).

---

## 📦 Single-binary build

```sh
make build   # frontend build + a Go binary with it embedded (bin/server)
make run     # build, then run it — one process, one port
```

See the [Makefile shortcuts](#-makefile-shortcuts) section above for how the
embedding works. `make public` does the same and additionally opens a public
tunnel to it (ngrok / cloudflared / localtunnel), handy for testing on a
phone or sharing a running instance without deploying anywhere.

---

## 🐳 Production build (Docker)

```sh
docker build -t lensrace .
docker run -p 3000:3000 -e DB_PATH=/data/lensrace.db -v lensrace-data:/data lensrace
```

The `Dockerfile` is a multi-stage build (Node to build the frontend, Go to
embed it and compile the binary, then a minimal Alpine runtime image) —
Render's native runtimes are single-language, so this repo needs Docker to
build both halves in one place. Same non-root/persistent-volume shape as
`docker run` above is what `render.yaml` configures for a real deploy.

### Deploying to Render

`render.yaml` defines a Docker web service with a persistent disk mounted
at `/data` (SQLite needs a real disk — Render's Free tier has none, so this
requires a paid instance type). From the Render dashboard: **New >
Blueprint**, point it at this repo, and it picks up `render.yaml`
automatically. Health checks hit `/api/health`; `PORT` is supplied by
Render and already read via `config.Load()`.

---

## 📁 Fiber CLI reference

| Command         | Description                                  |
| --------------- | -------------------------------------------- |
| `fiber new`     | Scaffold a new project from a template       |
| `fiber dev`     | Run with live reload                          |
| `fiber serve`   | Quick static file server                      |
| `fiber migrate` | Migrate between Fiber versions                |
| `fiber upgrade` | Update the CLI itself                         |
| `fiber version` | Show CLI and Fiber versions                   |

Docs: <https://docs.gofiber.io/blog/fiber-cli>
