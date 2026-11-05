import { createI18n } from 'vue-i18n'
import en from './locales/en.json'
import ru from './locales/ru.json'

export type AppLocale = 'en' | 'ru'

export const LANGUAGE_KEY = 'photo-bridge.language'

export function readStoredLanguage(): AppLocale {
  try {
    return localStorage.getItem(LANGUAGE_KEY) === 'ru' ? 'ru' : 'en'
  } catch {
    return 'en'
  }
}

export const i18n = createI18n({
  legacy: false,
  globalInjection: true,
  locale: readStoredLanguage(),
  fallbackLocale: 'en',
  messages: { en, ru },
})
