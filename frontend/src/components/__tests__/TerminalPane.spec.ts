import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import { createI18n } from 'vue-i18n'
import { useWebSocket } from '@/composables/useWebSocket'
import { useTerminal } from '@/composables/useTerminal'
import type { ClientMessage } from '@/types'
import en from '@/locales/en.json'
import TerminalPane from '../TerminalPane.vue'

vi.mock('@/composables/useWebSocket', () => ({ useWebSocket: vi.fn<typeof useWebSocket>() }))
vi.mock('@/composables/useTerminal', () => ({ useTerminal: vi.fn<typeof useTerminal>() }))
vi.mock('@/stores/config', () => ({
  useConfigStore: () => ({ scrollback: 5000 }),
}))
vi.mock('@/stores/settings', () => ({
  useSettingsStore: () => ({ fontSize: 14, fontFamily: '"JetBrains Mono", monospace' }),
}))

const i18n = createI18n({ legacy: false, locale: 'en', messages: { en } })

function makeUseTerminalReturn(initSpy = vi.fn<(el: HTMLElement, scrollback?: number, fontSize?: number, theme?: 'dark' | 'light', fontFamily?: string) => void>()) {
  return {
    init: initSpy,
    write: vi.fn<(data: string) => void>(),
    fit: vi.fn<() => { cols: number; rows: number } | null>(() => ({ cols: 80, rows: 24 })),
    onData: vi.fn<(handler: (data: string) => void) => undefined>(),
    search: vi.fn<(query: string) => void>(),
    searchPrev: vi.fn<(query: string) => void>(),
    setFontSize: vi.fn<(size: number) => void>(),
    setFontFamily: vi.fn<(family: string) => void>(),
    setTheme: vi.fn<(theme: 'dark' | 'light') => void>(),
    dispose: vi.fn<() => void>(),
    terminal: () => null,
    termRef: ref(null),
  }
}

describe('TerminalPane', () => {
  beforeEach(() => {
    vi.mocked(useWebSocket).mockReturnValue({
      connected: ref(true),
      send: vi.fn<(msg: ClientMessage) => void>(),
      disconnect: vi.fn<() => void>(),
    })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ mode: 'exec' }))))
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  function mountPane() {
    return mount(TerminalPane, {
      props: { sessionId: 'sess-abc', paneIndex: 0, isActive: true },
      global: { plugins: [i18n] },
    })
  }

  it('initializes the terminal with the configured font family', () => {
    const initSpy = vi.fn<(el: HTMLElement, scrollback?: number, fontSize?: number, theme?: 'dark' | 'light', fontFamily?: string) => void>()
    vi.mocked(useTerminal).mockReturnValue(makeUseTerminalReturn(initSpy))

    mountPane()

    expect(initSpy).toHaveBeenCalledWith(
      expect.anything(),
      5000,
      14,
      undefined,
      '"JetBrains Mono", monospace',
    )
  })

  it('exposes setFontFamily for parent-triggered font updates', () => {
    vi.mocked(useTerminal).mockReturnValue(makeUseTerminalReturn())

    const wrapper = mountPane()

    expect(typeof (wrapper.vm as unknown as { setFontFamily: (family: string) => void }).setFontFamily).toBe('function')
  })
})
