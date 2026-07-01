// Typed client for the LensRace/Snap Hunt backend API. Mirrors the JSON
// shapes in internal/models and internal/handlers.

export interface Category {
  id: string
  name: string
  description: string
}

export interface Game {
  id: string
  joinCode: string
  hostId?: string
  categoryId: string
  status: 'waiting' | 'countdown' | 'playing' | 'finished'
  durationSeconds: number
  createdAt: string
  startedAt?: string
  endedAt?: string
}

export interface Player {
  id: string
  gameId: string
  name: string
  isHost: boolean
  score: number
  capturedItemIds: string[]
  connectedAt: string
  disconnectedAt?: string
}

export interface Item {
  id: string
  categoryId: string
  label: string
  displayName: string
}

export interface Capture {
  id: string
  gameId: string
  playerId: string
  itemId: string
  confidence?: number
  capturedAt: string
}

export interface GameState {
  game: Game
  items: Item[]
  players: Player[]
}

export interface SessionState extends GameState {
  playerId: string
}

export interface CaptureResult extends GameState {
  capture: Capture
}

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`/api${path}`, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })

  if (!res.ok) {
    let message = res.statusText
    try {
      const data = (await res.json()) as { error?: string }
      if (data.error) message = data.error
    } catch {
      // response wasn't JSON; fall back to statusText
    }
    throw new ApiError(message, res.status)
  }

  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export function listCategories(): Promise<Category[]> {
  return request('GET', '/categories')
}

export function createGame(
  categoryId: string,
  hostName: string,
  durationSeconds?: number,
): Promise<SessionState> {
  return request('POST', '/games', { categoryId, hostName, durationSeconds })
}

export function joinGame(idOrCode: string, name: string): Promise<SessionState> {
  return request('POST', `/games/${encodeURIComponent(idOrCode)}/join`, { name })
}

export function getGame(idOrCode: string): Promise<GameState> {
  return request('GET', `/games/${encodeURIComponent(idOrCode)}`)
}

export function setCategory(
  idOrCode: string,
  playerId: string,
  categoryId: string,
): Promise<GameState> {
  return request('PATCH', `/games/${encodeURIComponent(idOrCode)}/category`, {
    playerId,
    categoryId,
  })
}

export function setDuration(
  idOrCode: string,
  playerId: string,
  durationSeconds: number,
): Promise<GameState> {
  return request('PATCH', `/games/${encodeURIComponent(idOrCode)}/duration`, {
    playerId,
    durationSeconds,
  })
}

export function startGame(idOrCode: string, playerId: string): Promise<GameState> {
  return request('POST', `/games/${encodeURIComponent(idOrCode)}/start`, { playerId })
}

export function recordCapture(
  idOrCode: string,
  playerId: string,
  itemId: string,
  confidence?: number,
): Promise<CaptureResult> {
  return request('POST', `/games/${encodeURIComponent(idOrCode)}/captures`, {
    playerId,
    itemId,
    confidence,
  })
}

/** Subscribes to the live game state stream. Returns an unsubscribe function. */
export function subscribeToGame(
  idOrCode: string,
  onState: (state: GameState) => void,
  onError?: (err: Event) => void,
): () => void {
  const source = new EventSource(`/api/games/${encodeURIComponent(idOrCode)}/events`)
  source.addEventListener('state', (event) => {
    onState(JSON.parse((event as MessageEvent).data) as GameState)
  })
  if (onError) source.addEventListener('error', onError)
  return () => source.close()
}
