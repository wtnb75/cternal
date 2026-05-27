import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSessionStore } from '../session'

const mockSession = {
  id: 'sess-1',
  containerId: 'ctr-1',
  mode: 'exec' as const,
  status: 'active' as const,
  wsUrl: 'ws://localhost/ws/sess-1',
}

describe('useSessionStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  describe('fetchSessions', () => {
    it('populates sessions list', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify([mockSession])))
      const store = useSessionStore()
      await store.fetchSessions()
      expect(store.sessions).toHaveLength(1)
      expect(store.sessions[0]!.id).toBe('sess-1')
    })

    it('sets empty list when no sessions exist', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response(JSON.stringify([])))
      const store = useSessionStore()
      await store.fetchSessions()
      expect(store.sessions).toHaveLength(0)
    })
  })

  describe('createSession', () => {
    it('appends the new session and returns it', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(
        new Response(JSON.stringify(mockSession), { status: 201 }),
      )
      const store = useSessionStore()
      const sess = await store.createSession({ containerId: 'ctr-1', mode: 'exec' })
      expect(sess.id).toBe('sess-1')
      expect(store.sessions).toHaveLength(1)
    })

    it('throws the error message on 5xx response', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(
        new Response(JSON.stringify({ error: 'container not running' }), { status: 500 }),
      )
      const store = useSessionStore()
      await expect(store.createSession({ containerId: 'ctr-1', mode: 'exec' })).rejects.toThrow(
        'container not running',
      )
    })

    it('throws fallback message when response has no error field', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(
        new Response(JSON.stringify({}), { status: 500 }),
      )
      const store = useSessionStore()
      await expect(store.createSession({ containerId: 'ctr-1', mode: 'exec' })).rejects.toThrow(
        'Failed to create session',
      )
    })
  })

  describe('deleteSession', () => {
    it('removes session from list', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response(null, { status: 204 }))
      const store = useSessionStore()
      store.sessions = [mockSession]
      await store.deleteSession('sess-1')
      expect(store.sessions).toHaveLength(0)
    })

    it('clears currentSession when the deleted session is active', async () => {
      vi.mocked(fetch).mockResolvedValueOnce(new Response(null, { status: 204 }))
      const store = useSessionStore()
      store.sessions = [mockSession]
      store.setCurrentSession(mockSession)
      await store.deleteSession('sess-1')
      expect(store.currentSession).toBeNull()
    })

    it('preserves currentSession when a different session is deleted', async () => {
      const other = { ...mockSession, id: 'sess-2' }
      vi.mocked(fetch).mockResolvedValueOnce(new Response(null, { status: 204 }))
      const store = useSessionStore()
      store.sessions = [mockSession, other]
      store.setCurrentSession(other)
      await store.deleteSession('sess-1')
      expect(store.currentSession?.id).toBe('sess-2')
    })
  })

  describe('setCurrentSession', () => {
    it('sets the current session', () => {
      const store = useSessionStore()
      store.setCurrentSession(mockSession)
      expect(store.currentSession?.id).toBe('sess-1')
    })

    it('clears the current session when set to null', () => {
      const store = useSessionStore()
      store.setCurrentSession(mockSession)
      store.setCurrentSession(null)
      expect(store.currentSession).toBeNull()
    })
  })
})
