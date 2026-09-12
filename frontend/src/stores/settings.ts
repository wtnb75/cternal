import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { DEFAULT_TERMINAL_FONT } from '@/constants/fonts'

type Theme = 'dark' | 'light'
type Language = 'ja' | 'en'

function load<T>(key: string, fallback: T): T {
  try {
    const v = localStorage.getItem(key)
    return v !== null ? (JSON.parse(v) as T) : fallback
  } catch {
    return fallback
  }
}

export const useSettingsStore = defineStore('settings', () => {
  const theme = ref<Theme>(load('cternal.theme', 'dark'))
  const language = ref<Language>(load('cternal.language', 'ja'))
  const fontSize = ref<number>(load('cternal.fontSize', 14))
  const fontFamily = ref<string>(load('cternal.fontFamily', DEFAULT_TERMINAL_FONT))

  watch(theme, v => {
    localStorage.setItem('cternal.theme', JSON.stringify(v))
    applyTheme()
  })
  watch(language, v => localStorage.setItem('cternal.language', JSON.stringify(v)))
  watch(fontSize, v => localStorage.setItem('cternal.fontSize', JSON.stringify(v)))
  watch(fontFamily, v => localStorage.setItem('cternal.fontFamily', JSON.stringify(v)))

  function applyTheme() {
    document.documentElement.setAttribute('data-theme', theme.value)
  }

  return { theme, language, fontSize, fontFamily, applyTheme }
})
