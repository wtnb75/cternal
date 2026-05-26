import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import { createRouter, createMemoryHistory } from 'vue-router'
import { useWebSocket } from '@/composables/useWebSocket'
import { useTerminal } from '@/composables/useTerminal'
import TerminalView from '../TerminalView.vue'

vi.mock('@/composables/useWebSocket', () => ({ useWebSocket: vi.fn() }))
vi.mock('@/composables/useTerminal', () => ({ useTerminal: vi.fn() }))

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
  let mockSend: ReturnType<typeof vi.fn>
  let mockConnected: ReturnType<typeof ref<boolean>>

  beforeEach(() => {
    mockSend = vi.fn()
    mockConnected = ref(true)

    vi.mocked(useWebSocket).mockReturnValue({
      connected: mockConnected,
      send: mockSend,
      disconnect: vi.fn(),
    })
    vi.mocked(useTerminal).mockReturnValue({
      init: vi.fn(),
      write: vi.fn(),
      fit: vi.fn(() => ({ cols: 80, rows: 24 })),
      onData: vi.fn(),
      search: vi.fn(),
      dispose: vi.fn(),
      terminal: () => null,
      termRef: ref(null),
    })

    vi.stubGlobal('fetch', vi.fn())
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
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ mode: 'exec' })),
    )
    const wrapper = await mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('exec')
  })

  it('download button triggers cast fetch', async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(new Response(JSON.stringify({ mode: 'exec' })))
      .mockResolvedValueOnce(new Response(new Blob(['cast-data'])))
    vi.stubGlobal('URL', { createObjectURL: vi.fn(() => 'blob:fake') })

    const wrapper = await mountView('sess-abc')
    await flushPromises()
    await wrapper.find('button.btn-sm').trigger('click')
    await flushPromises()

    expect(fetch).toHaveBeenCalledWith('/api/v1/sessions/sess-abc/cast')
  })

  it('sends resize message on mount', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify({ mode: 'exec' })))
    await mountView()
    await flushPromises()
    expect(mockSend).toHaveBeenCalledWith(
      expect.objectContaining({ type: 'resize' }),
    )
  })

  it('back button navigates to /', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify({ mode: 'exec' })))
    const wrapper = await mountView()
    await wrapper.find('button.btn-back').trigger('click')
    await flushPromises() // router.push is async
    expect(wrapper.vm.$router.currentRoute.value.path).toBe('/')
  })
})
