// Cosmetic emoji per category. The backend doesn't model an icon, so this
// is a small client-side lookup with a sensible fallback for any category
// added later without a matching entry here.
const ICONS: Record<string, string> = {
  'house-essentials': '🏠',
  'city-scavenger': '🌆',
}

export function categoryEmoji(categoryId: string): string {
  return ICONS[categoryId] ?? '🎯'
}
