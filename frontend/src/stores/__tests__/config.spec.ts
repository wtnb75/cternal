import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useConfigStore } from '../config'

describe('useConfigStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('defaults to empty username and logoutUrl', () => {
    const store = useConfigStore()
    expect(store.username).toBe('')
    expect(store.logoutUrl).toBe('')
  })

  it('populates username and logoutUrl from /api/v1/config', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ scrollback: 1000, username: 'alice', logoutUrl: '/oauth2/sign_out' })),
    )
    const store = useConfigStore()
    await store.load()
    expect(store.username).toBe('alice')
    expect(store.logoutUrl).toBe('/oauth2/sign_out')
  })

  it('leaves username and logoutUrl empty when absent from response', async () => {
    vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify({ scrollback: 1000 })))
    const store = useConfigStore()
    await store.load()
    expect(store.username).toBe('')
    expect(store.logoutUrl).toBe('')
  })
})
