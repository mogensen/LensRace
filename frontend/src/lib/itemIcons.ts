// Cosmetic emoji per item, keyed by the item's COCO-SSD label. A generic
// fallback covers any label added later without a matching entry here.
const ICONS: Record<string, string> = {
  chair: '🪑',
  cup: '🥤',
  book: '📖',
  clock: '🕐',
  laptop: '💻',
  bottle: '🧴',
  car: '🚗',
  bicycle: '🚲',
  'traffic light': '🚦',
  dog: '🐶',
  backpack: '🎒',
  bench: '🛋️',
  keyboard: '⌨️',
  remote: '📺',
  vase: '🏺',
  scissors: '✂️',
  'teddy bear': '🧸',
  'potted plant': '🪴',
  bus: '🚌',
  truck: '🚚',
  motorcycle: '🏍️',
  'stop sign': '🛑',
  'fire hydrant': '🚒',
  umbrella: '☂️',
}

export function itemEmoji(label: string): string {
  return ICONS[label] ?? '📦'
}
