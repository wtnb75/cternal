import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import type { Mock } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import type { Ref } from 'vue'
import { createRouter, createMemoryHistory } from 'vue-router'
import { useWebSocket } from '@/composables/useWebSocket'
import { useTerminal } from '@/composables/useTerminal'
import type { ClientMessage, WSMessage } from '@/types'
import TerminalView from '../TerminalView.vue'

vi.mock('@/composables/useWebSocket', () => ({ useWebSocket: vi.fn<typeof useWebSocket>() }))
vi.mock('@/composables/useTerminal', () => ({ useTerminal: vi.fn<typeof useTerminal>() }))
vi.mock('@/stores/config', () => ({
  useConfigStore: () => ({ scrollback: 5000, load: vi.fn<() => Promise<void>>() }),
}))

function makeRouter(id = 'sess-abc') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div/>' } },
      { path: '/sessions/:id', name: 'terminal', component: TerminalView },
    ],
  })
  router.push(`/sessions/${id}`)
  return router
}

describe('TerminalView', () => {
  let mockSend: Mock<(msg: ClientMessage) => void>
  let mockConnected: Ref<boolean>

  beforeEach(() => {
    mockSend = vi.fn<(msg: ClientMessage) => void>()
    mockConnected = ref<boolean>(true)

    vi.mocked(useWebSocket).mockReturnValue({
      connected: mockConnected,
      send: mockSend,
      disconnect: vi.fn<() => void>(),
    })
    vi.mocked(useTerminal).mockReturnValue({
      init: vi.fn<(el: HTMLElement, scrollback?: number) => void>(),
      write: vi.fn<(data: string) => void>(),
      fit: vi.fn<() => { cols: number; rows: number } | null>(() => ({ cols: 80, rows: 24 })),
      onData: vi.fn<(handler: (data: string) => void) => undefined>(),
      search: vi.fn<(query: string) => void>(),
      dispose: vi.fn<() => void>(),
      terminal: () => null,
      termRef: ref(null),
    })

    vi.stubGlobal('fetch', vi.fn<typeof fetch>())
    vi.spyOn(console, 'warn').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  async function mountView(id = 'sess-abc') {
    const router = makeRouter(id)
    await router.isReady()
    return mount(TerminalView, { global: { plugins: [router] } })
  }

  it('displays the session ID in the toolbar', async () => {
    const wrapper = await mountView('sess-abc')
    expect(wrapper.text()).toContain('sess-abc')
  })

  it('shows "Connected" when connected is true', async () => {
    mockConnected.value = true
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('Connected')
  })

  it('shows "Reconnecting" when connected is false', async () => {
    mockConnected.value = false
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('Reconnecting')
  })

  it('fetches session info on mount to display mode', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify({ mode: 'exec' })))
    const wrapper = await mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('exec')
  })

  it('download button triggers cast fetch', async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(new Response(JSON.stringify({ mode: 'exec' })))
      .mockResolvedValueOnce(new Response(new Blob(['cast-data'])))
    vi.stubGlobal('URL', { createObjectURL: vi.fn<() => string>(() => 'blob:fake') })

    const wrapper = await mountView('sess-abc')
    await flushPromises()
    await wrapper.find('button.btn-download').trigger('click')
    await flushPromises()

    expect(fetch).toHaveBeenCalledWith('/api/v1/sessions/sess-abc/cast')
  })

  it('sends resize message on mount', async () => {
    await mountView()
    expect(mockSend).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'resize' }),
    )
  })

  it('writes exit banner to terminal when process exits', async () => {
    // Capture the WS message callback and terminal write spy via mockImplementationOnce.
    let wsHandler: ((msg: WSMessage) => void) = () => {}
    vi.mocked(useWebSocket).mockImplementationOnce((_url, cb) => {
      wsHandler = cb
      return { connected: mockConnected, send: mockSend, disconnect: vi.fn<() => void>() }
    })

    const termWrite = vi.fn<(data: string) => void>()
    vi.mocked(useTerminal).mockImplementationOnce(() => ({
      init: vi.fn<(el: HTMLElement, scrollback?: number) => void>(),
      write: termWrite,
      fit: vi.fn<() => { cols: number; rows: number } | null>(() => ({ cols: 80, rows: 24 })),
      onData: vi.fn<(handler: (data: string) => void) => undefined>(),
      search: vi.fn<(query: string) => void>(),
      dispose: vi.fn<() => void>(),
      terminal: () => null,
      termRef: ref(null),
    }))

    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify({ mode: 'exec' })))
    await mountView()
    await flushPromises()

    wsHandler({ type: 'exit' })

    expect(termWrite).toHaveBeenCalledWith(expect.stringContaining('process exited'))
  })
})
