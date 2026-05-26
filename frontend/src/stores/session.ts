import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Session, CreateSessionRequest } from '@/types'

export const useSessionStore = defineStore('session', () => {
  const sessions = ref<Session[]>([])
  const currentSession = ref<Session | null>(null)

  async function fetchSessions() {
    const res = await fetch('/api/v1/sessions')
    sessions.value = await res.json()
  }

  async function createSession(req: CreateSessionRequest): Promise<Session> {
    const res = await fetch('/api/v1/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
    })
    if (!res.ok) {
      const err = await res.json()
      throw new Error(err.error ?? 'Failed to create session')
    }
    const sess: Session = await res.json()
    sessions.value.push(sess)
    return sess
  }

  async function deleteSession(id: string) {
    await fetch(`/api/v1/sessions/${id}`, { method: 'DELETE' })
    sessions.value = sessions.value.filter(s => s.id !== id)
    if (currentSession.value?.id === id) currentSession.value = null
  }

  function setCurrentSession(sess: Session | null) {
    currentSession.value = sess
  }

  return { sessions, currentSession, fetchSessions, createSession, deleteSession, setCurrentSession }
})
