// Reactive singleton wrapping the game API + live SSE state + local session
// persistence. There's no auth yet, so "who am I" is just a playerId we
// stash in localStorage alongside the game it belongs to.
import { reactive, computed, readonly } from 'vue'
import * as api from '@/lib/api'
import type { GameState, Player } from '@/lib/api'

const STORAGE_KEY = 'snaphunt.session'

interface StoredSession {
  gameId: string
  playerId: string
}

function loadSession(): StoredSession | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? (JSON.parse(raw) as StoredSession) : null
  } catch {
    return null
  }
}

function saveSession(session: StoredSession | null) {
  if (session) localStorage.setItem(STORAGE_KEY, JSON.stringify(session))
  else localStorage.removeItem(STORAGE_KEY)
}

const state = reactive({
  playerId: '',
  gameState: null as GameState | null,
  connected: false,
})

let unsubscribe: (() => void) | null = null

function connect(gameId: string) {
  unsubscribe?.()
  unsubscribe = api.subscribeToGame(
    gameId,
    (gs) => {
      state.gameState = gs
      state.connected = true
    },
    () => {
      state.connected = false
    },
  )
}

function disconnect() {
  unsubscribe?.()
  unsubscribe = null
  state.connected = false
}

async function createGame(categoryId: string, hostName: string, durationSeconds?: number) {
  const session = await api.createGame(categoryId, hostName, durationSeconds)
  state.playerId = session.playerId
  state.gameState = session
  saveSession({ gameId: session.game.id, playerId: session.playerId })
  connect(session.game.id)
  return session
}

async function joinGame(codeOrId: string, name: string) {
  const session = await api.joinGame(codeOrId, name)
  state.playerId = session.playerId
  state.gameState = session
  saveSession({ gameId: session.game.id, playerId: session.playerId })
  connect(session.game.id)
  return session
}

/**
 * Restores the session for gameId if we already have it in memory or in
 * localStorage from a previous visit. Returns false when this browser has
 * no known identity for that game (caller should send the user home).
 */
async function ensureSession(gameId: string): Promise<boolean> {
  if (state.gameState?.game.id === gameId && state.playerId) {
    if (!state.connected) connect(gameId)
    return true
  }

  const stored = loadSession()
  if (!stored || stored.gameId !== gameId) return false

  try {
    state.playerId = stored.playerId
    state.gameState = await api.getGame(gameId)
    connect(gameId)
    return true
  } catch {
    return false
  }
}

async function changeCategory(categoryId: string) {
  if (!state.gameState) return
  await api.setCategory(state.gameState.game.id, state.playerId, categoryId)
}

async function changeDuration(durationSeconds: number) {
  if (!state.gameState) return
  await api.setDuration(state.gameState.game.id, state.playerId, durationSeconds)
}

async function start() {
  if (!state.gameState) return
  await api.startGame(state.gameState.game.id, state.playerId)
}

async function capture(itemId: string, confidence?: number) {
  if (!state.gameState) return
  return api.recordCapture(state.gameState.game.id, state.playerId, itemId, confidence)
}

function reset() {
  disconnect()
  state.playerId = ''
  state.gameState = null
  saveSession(null)
}

const me = computed<Player | undefined>(() =>
  state.gameState?.players.find((p) => p.id === state.playerId),
)
const isHost = computed(() => me.value?.isHost ?? false)

export function useGameStore() {
  // Wrapped in reactive() so nested refs (me, isHost) auto-unwrap on
  // property access in templates, the same way Pinia stores do — a plain
  // object here would hand templates the raw ComputedRef instead of its
  // value.
  return reactive({
    state: readonly(state),
    me,
    isHost,
    createGame,
    joinGame,
    ensureSession,
    changeCategory,
    changeDuration,
    start,
    capture,
    reset,
  })
}
