import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { nextTick } from 'vue'
import { useSettingsStore } from '../settings'

describe('useSettingsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
  })

  afterEach(() => {
    document.documentElement.removeAttribute('data-theme')
  })

  describe('defaults', () => {
    it('defaults theme to dark', () => {
      expect(useSettingsStore().theme).toBe('dark')
    })

    it('defaults language to ja', () => {
      expect(useSettingsStore().language).toBe('ja')
    })

    it('defaults fontSize to 14', () => {
      expect(useSettingsStore().fontSize).toBe(14)
    })
  })

  describe('loading from localStorage', () => {
    it('loads theme from localStorage', () => {
      localStorage.setItem('cternal.theme', '"light"')
      expect(useSettingsStore().theme).toBe('light')
    })

    it('loads language from localStorage', () => {
      localStorage.setItem('cternal.language', '"en"')
      expect(useSettingsStore().language).toBe('en')
    })

    it('loads fontSize from localStorage', () => {
      localStorage.setItem('cternal.fontSize', '18')
      expect(useSettingsStore().fontSize).toBe(18)
    })

    it('falls back to default on invalid JSON', () => {
      localStorage.setItem('cternal.theme', '{invalid')
      expect(useSettingsStore().theme).toBe('dark')
    })
  })

  describe('persistence to localStorage', () => {
    it('persists theme change', async () => {
      const store = useSettingsStore()
      store.theme = 'light'
      await nextTick()
      expect(localStorage.getItem('cternal.theme')).toBe('"light"')
    })

    it('persists language change', async () => {
      const store = useSettingsStore()
      store.language = 'en'
      await nextTick()
      expect(localStorage.getItem('cternal.language')).toBe('"en"')
    })

    it('persists fontSize change', async () => {
      const store = useSettingsStore()
      store.fontSize = 20
      await nextTick()
      expect(localStorage.getItem('cternal.fontSize')).toBe('20')
    })
  })

  describe('applyTheme', () => {
    it('sets data-theme attribute on documentElement', () => {
      const store = useSettingsStore()
      store.applyTheme()
      expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
    })

    it('reflects the current theme value', () => {
      const store = useSettingsStore()
      store.theme = 'light'
      store.applyTheme()
      expect(document.documentElement.getAttribute('data-theme')).toBe('light')
    })
  })
})
