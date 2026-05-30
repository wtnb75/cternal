import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import { createI18n } from 'vue-i18n'
import en from '@/locales/en.json'
import App from '../App.vue'

vi.mock('@/components/ContainerSidebar.vue', () => ({
  default: { template: '<div class="stub-sidebar" />' },
}))
vi.mock('@/stores/config', () => ({
  useConfigStore: () => ({ load: vi.fn<() => void>(), scrollback: 5000 }),
}))
vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({
    applyTheme: vi.fn<() => void>(),
    language: 'en',
    fontSize: 14,
    theme: 'dark',
  }),
}))

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } })

function makeRouter() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', component: { template: '<div/>' } }],
  })
  router.push('/')
  return router
}

async function mountApp() {
  const router = makeRouter()
  const wrapper = mount(App, { global: { plugins: [router, i18n] } })
  await flushPromises()
  return wrapper
}

describe('App sidebar collapse', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    localStorage.clear()
  })

  it('starts expanded when localStorage is empty', async () => {
    const wrapper = await mountApp()
    expect(wrapper.find('aside.sidebar').classes()).not.toContain('collapsed')
  })

  it('starts collapsed when localStorage has sidebar_collapsed=1', async () => {
    localStorage.setItem('sidebar_collapsed', '1')
    const wrapper = await mountApp()
    expect(wrapper.find('aside.sidebar').classes()).toContain('collapsed')
  })

  it('toggles collapsed class when toggle button is clicked', async () => {
    const wrapper = await mountApp()
    const btn = wrapper.find('button.sidebar-toggle')
    expect(wrapper.find('aside.sidebar').classes()).not.toContain('collapsed')

    await btn.trigger('click')
    expect(wrapper.find('aside.sidebar').classes()).toContain('collapsed')

    await btn.trigger('click')
    expect(wrapper.find('aside.sidebar').classes()).not.toContain('collapsed')
  })

  it('persists collapsed state to localStorage on toggle', async () => {
    const wrapper = await mountApp()
    await wrapper.find('button.sidebar-toggle').trigger('click')
    expect(localStorage.getItem('sidebar_collapsed')).toBe('1')

    await wrapper.find('button.sidebar-toggle').trigger('click')
    expect(localStorage.getItem('sidebar_collapsed')).toBe('0')
  })

  it('focuses active terminal pane on toggle when present', async () => {
    const wrapper = await mountApp()
    const mockFocus = vi.fn<() => void>()
    const textarea = document.createElement('textarea')
    textarea.className = 'xterm-helper-textarea'
    const pane = document.createElement('div')
    pane.className = 'terminal-pane active'
    pane.appendChild(textarea)
    document.body.appendChild(pane)
    textarea.focus = mockFocus

    await wrapper.find('button.sidebar-toggle').trigger('click')
    await flushPromises()

    expect(mockFocus).toHaveBeenCalled()
    document.body.removeChild(pane)
  })

  it('falls back to any terminal when no active pane exists', async () => {
    const wrapper = await mountApp()
    const mockFocus = vi.fn<() => void>()
    const textarea = document.createElement('textarea')
    textarea.className = 'xterm-helper-textarea'
    document.body.appendChild(textarea)
    textarea.focus = mockFocus

    await wrapper.find('button.sidebar-toggle').trigger('click')
    await flushPromises()

    expect(mockFocus).toHaveBeenCalled()
    document.body.removeChild(textarea)
  })

  it('does not throw when no terminal is present on toggle', async () => {
    const wrapper = await mountApp()
    await expect(wrapper.find('button.sidebar-toggle').trigger('click')).resolves.not.toThrow()
  })
})
