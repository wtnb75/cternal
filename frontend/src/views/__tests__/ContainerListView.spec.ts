import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import ContainerListView from '../ContainerListView.vue'

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: ContainerListView },
      { path: '/sessions/:id', name: 'terminal', component: { template: '<div/>' } },
    ],
  })
}

const containers = [
  { id: 'abc123def456', name: 'web', image: 'nginx:latest', status: 'running', running: true },
  { id: 'def456abc123', name: '', image: 'postgres:15', status: 'exited', running: false },
]

describe('ContainerListView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  function mountView() {
    return mount(ContainerListView, {
      global: { plugins: [makeRouter(), createPinia()] },
    })
  }

  it('calls /api/v1/containers on mount', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify([])))
    mountView()
    await flushPromises()
    expect(fetch).toHaveBeenCalledWith(expect.stringContaining('/api/v1/containers'))
  })

  it('renders container name and image', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify(containers)))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('web')
    expect(wrapper.text()).toContain('nginx:latest')
  })

  it('falls back to id prefix when container has no name', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify(containers)))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('def456abc123'.slice(0, 12))
  })

  it('shows "No containers found" when list is empty', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify([])))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('No containers found')
  })

  it('shows error message on fetch failure', async () => {
    vi.mocked(fetch).mockRejectedValueOnce(new Error('network error'))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('Failed to fetch containers')
  })

  it('Exec and Attach buttons are disabled for stopped containers', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify(containers)))
    const wrapper = mountView()
    await flushPromises()
    const rows = wrapper.findAll('tbody tr')
    const btns = rows[1]!.findAll('button') // second row = stopped
    expect(btns[0]!.element.disabled).toBe(true)  // Exec
    expect(btns[1]!.element.disabled).toBe(true)  // Attach
    expect(btns[2]!.element.disabled).toBe(false) // Logs always enabled
  })

  it('Exec and Attach buttons are enabled for running containers', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify(containers)))
    const wrapper = mountView()
    await flushPromises()
    const rows = wrapper.findAll('tbody tr')
    const btns = rows[0]!.findAll('button') // first row = running
    expect(btns[0]!.element.disabled).toBe(false) // Exec
    expect(btns[1]!.element.disabled).toBe(false) // Attach
  })

  it('navigates to terminal view after successful connect', async () => {
    const sess = { id: 's1', containerId: 'abc123def456', mode: 'exec', status: 'active', wsUrl: '' }
    vi.mocked(fetch)
      .mockResolvedValueOnce(new Response(JSON.stringify(containers)))
      .mockResolvedValueOnce(new Response(JSON.stringify(sess), { status: 201 }))
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('tbody tr')[0]!.findAll('button')[0]!.trigger('click')
    await flushPromises()
    expect(wrapper.vm.$router.currentRoute.value.params.id).toBe('s1')
  })

  it('shows server error message when createSession fails', async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(new Response(JSON.stringify(containers)))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'container not running' }), { status: 500 }),
      )
    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('tbody tr')[0]!.findAll('button')[0]!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('container not running')
  })

  it('appends name filter param to fetch URL', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response(JSON.stringify([])))
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('input.filter-input').setValue('nginx')
    await flushPromises()
    expect(fetch).toHaveBeenCalledWith(expect.stringContaining('name=nginx'))
  })

  it('appends status filter param to fetch URL', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response(JSON.stringify([])))
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('select.filter-select').setValue('running')
    await flushPromises()
    expect(fetch).toHaveBeenCalledWith(expect.stringContaining('status=running'))
  })

  it('Refresh button triggers a new fetch', async () => {
    vi.mocked(fetch).mockResolvedValue(new Response(JSON.stringify([])))
    const wrapper = mountView()
    await flushPromises()
    await wrapper.find('button.btn').trigger('click')
    await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(2)
  })
})
