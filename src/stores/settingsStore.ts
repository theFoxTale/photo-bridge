import { defineStore } from 'pinia'
import { ref } from 'vue'
import { i18n, LANGUAGE_KEY, type AppLocale } from '../i18n'

export type Theme = 'light' | 'dark'
export type Language = AppLocale

const THEME_KEY = 'photo-bridge.theme'

function readTheme(): Theme {
  try {
    return localStorage.getItem(THEME_KEY) === 'light' ? 'light' : 'dark'
  } catch {
    return 'dark'
  }
}

function readLanguage(): Language {
  try {
    return localStorage.getItem(LANGUAGE_KEY) === 'ru' ? 'ru' : 'en'
  } catch {
    return 'en'
  }
}

function applyTheme(theme: Theme): void {
  document.documentElement.classList.toggle('dark', theme === 'dark')
}

function applyLanguage(language: Language): void {
  i18n.global.locale.value = language
  document.documentElement.lang = language
}

export const useSettingsStore = defineStore('settings', () => {
  const theme = ref<Theme>(readTheme())
  const language = ref<Language>(readLanguage())

  applyTheme(theme.value)
  applyLanguage(language.value)

  function setTheme(next: Theme): void {
    if (next !== 'light' && next !== 'dark') return
    theme.value = next
    applyTheme(next)
    try {
      localStorage.setItem(THEME_KEY, next)
    } catch {
      // The theme still applies for this session when storage is blocked.
    }
  }

  function setLanguage(next: Language): void {
    if (next !== 'en' && next !== 'ru') return
    language.value = next
    applyLanguage(next)
    try {
      localStorage.setItem(LANGUAGE_KEY, next)
    } catch {
      // The language still applies for this session when storage is blocked.
    }
  }

  return { theme, language, setTheme, setLanguage }
})
