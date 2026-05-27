import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import type { Mock } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref, nextTick } from 'vue'
import { createRouter, createMemoryHistory } from 'vue-router'
import { useTerminal } from '@/composables/useTerminal'
import ReplayView from '../ReplayView.vue'

vi.mock('@/composables/useTerminal', () => ({ useTerminal: vi.fn<typeof useTerminal>() }))

const eventsFixture = [
  { Time: 0,          Type: 'o', Data: 'hello ' },
  { Time: 500_000_000, Type: 'o', Data: 'world' },  // 500ms gap (nanoseconds)
  { Time: 600_000_000, Type: 'o', Data: '!' },
]

function makeRouter(id = 'sess-replay') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/sessions/:id', name: 'terminal', component: { template: '<div/>' } },
      { path: '/sessions/:id/replay', name: 'replay', component: ReplayView },
    ],
  })
  router.push(`/sessions/${id}/replay`)
  return router
}

describe('ReplayView', () => {
  let mockWrite: Mock<(data: string) => void>
  let mockFit: Mock<() => { cols: number; rows: number } | null>
  let mockInit: Mock<(el: HTMLElement, scrollback?: number) => void>

  beforeEach(() => {
    mockWrite = vi.fn<(data: string) => void>()
    mockFit = vi.fn<() => { cols: number; rows: number } | null>(() => ({ cols: 80, rows: 24 }))
    mockInit = vi.fn<(el: HTMLElement, scrollback?: number) => void>()

    vi.mocked(useTerminal).mockReturnValue({
      init: mockInit,
      write: mockWrite,
      fit: mockFit,
      onData: vi.fn<(handler: (data: string) => void) => undefined>(),
      search: vi.fn<(query: string) => void>(),
      dispose: vi.fn<() => void>(),
      terminal: () => null,
      termRef: ref(null),
    })

    vi.stubGlobal('fetch', vi.fn<typeof fetch>())
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  async function mountView(id = 'sess-replay') {
    const router = makeRouter(id)
    await router.isReady()
    return mount(ReplayView, { global: { plugins: [router] }, attachTo: document.body })
  }

  it('displays session ID in the toolbar', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify([])))
    const wrapper = await mountView('sess-replay')
    await flushPromises()
    expect(wrapper.text()).toContain('sess-replay')
  })

  it('shows event count after loading', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify(eventsFixture)))
    const wrapper = await mountView()
    await flushPromises()
    expect(wrapper.text()).toContain(`/${eventsFixture.length}`)
  })

  it('writes output events to terminal on play', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify(eventsFixture)))
    const wrapper = await mountView()
    await flushPromises()

    await wrapper.find('button.btn:not(.btn-back)').trigger('click') // Play
    expect(mockWrite).toHaveBeenCalledWith('hello ')

    vi.advanceTimersByTime(500)
    expect(mockWrite).toHaveBeenCalledWith('world')
  })

  it('Play button label toggles between Play and Pause', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify(eventsFixture)))
    const wrapper = await mountView()
    await flushPromises()

    const btn = wrapper.find('button.btn:not(.btn-back)')
    expect(btn.text()).toBe('Play')
    await btn.trigger('click')
    expect(btn.text()).toBe('Pause')
    await btn.trigger('click')
    expect(btn.text()).toBe('Play')
  })

  it('stops at the end of events automatically', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify(eventsFixture)))
    const wrapper = await mountView()
    await flushPromises()

    await wrapper.find('button.btn:not(.btn-back)').trigger('click')
    vi.advanceTimersByTime(2000) // fast-forward through all events
    await nextTick() // let Vue update the DOM after reactive state change

    const btn = wrapper.find('button.btn:not(.btn-back)')
    expect(btn.text()).toBe('Play')
  })

  it('speed selector defaults to 1×', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify([])))
    const wrapper = await mountView()
    await flushPromises()
    const select = wrapper.find('select.speed-select')
    expect((select.element as HTMLSelectElement).value).toBe('1')
  })

  it('seek bar starts at 0', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify(eventsFixture)))
    const wrapper = await mountView()
    await flushPromises()
    const bar = wrapper.find('input[type="range"]')
    expect((bar.element as HTMLInputElement).value).toBe('0')
  })

  it('back button navigates to terminal view', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify([])))
    const wrapper = await mountView('sess-replay')
    await flushPromises()
    await wrapper.find('button.btn-back').trigger('click')
    await flushPromises() // router.push is async
    expect(wrapper.vm.$router.currentRoute.value.path).toBe('/sessions/sess-replay')
  })

  it('shows 0/0 when events array is empty', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify([])))
    const wrapper = await mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('0/0')
  })

  it('handles fetch failure gracefully without crashing', async () => {
    vi.mocked(fetch).mockRejectedValueOnce(new Error('network'))
    const wrapper = await mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('0/0')
  })
})
