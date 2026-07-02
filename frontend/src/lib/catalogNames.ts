// Categories and items come from the (untranslated) backend by stable ID.
// These look up the localized display name from the active locale's
// `categories`/`items` catalogs, falling back to the server-provided
// name/label for anything not yet translated.
type Translate = (key: string) => string
type TranslationExists = (key: string) => boolean

export function categoryName(t: Translate, te: TranslationExists, id: string, fallback: string): string {
  const key = `categories.${id}`
  return te(key) ? t(key) : fallback
}

export function itemName(t: Translate, te: TranslationExists, id: string, fallback: string): string {
  const key = `items.${id}`
  return te(key) ? t(key) : fallback
}
