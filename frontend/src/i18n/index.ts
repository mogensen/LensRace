import { createI18n } from 'vue-i18n'
import en from './locales/en'
import da from './locales/da'

export type SupportedLocale = 'en' | 'da'
export const SUPPORTED_LOCALES: SupportedLocale[] = ['en', 'da']

function isSupportedLocale(code: string): code is SupportedLocale {
  return (SUPPORTED_LOCALES as string[]).includes(code)
}

// Picks the first of the browser's preferred languages (in priority order)
// that we have translations for, falling back to English — this is what
// "use the browser locale" means when the browser prefers several
// languages we don't support before one we do (e.g. ['fr', 'da', 'en']).
export function detectLocale(): SupportedLocale {
  const languages = navigator.languages?.length ? navigator.languages : [navigator.language]
  for (const lang of languages) {
    const code = lang.split('-')[0]?.toLowerCase()
    if (code && isSupportedLocale(code)) return code
  }
  return 'en'
}

export const i18n = createI18n({
  legacy: false,
  locale: detectLocale(),
  fallbackLocale: 'en',
  messages: { en, da },
})
