// Deterministic per-player avatar emoji/color, derived from the player id.
// The backend has no notion of avatars — this just gives each player a
// consistent-looking identity across the lobby/play/results screens.
const EMOJIS = ['🦊', '🐼', '🐯', '🐨', '🐸', '🦁', '🐵', '🐰', '🐺', '🐷']
const COLORS = [
  '#ff6a3d',
  '#4aa3ff',
  '#ffc02e',
  '#9a6bff',
  '#34b36a',
  '#ff6bab',
  '#5ec8c8',
  '#e08a3c',
]

function hash(id: string): number {
  let h = 0
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0
  return h
}

export function avatarEmoji(id: string): string {
  return EMOJIS[hash(id) % EMOJIS.length]!
}

export function avatarColor(id: string): string {
  return COLORS[hash(id) % COLORS.length]!
}
