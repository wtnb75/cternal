<template>
  <div class="terminal-view">
    <div class="toolbar">
      <button @click="router.push('/')" class="btn btn-back">← Containers</button>
      <span class="session-info">Session: {{ sessionId }} · {{ mode }}</span>
      <span :class="['status', connected ? 'connected' : 'disconnected']">
        {{ connected ? 'Connected' : 'Reconnecting…' }}
      </span>
      <button @click="downloadCast" class="btn btn-sm">Download .cast</button>
    </div>
    <div ref="termEl" class="terminal-container" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTerminal } from '@/composables/useTerminal'
import { useWebSocket } from '@/composables/useWebSocket'
import type { WSMessage } from '@/types'

const route = useRoute()
const router = useRouter()
const sessionId = route.params.id as string

const termEl = ref<HTMLElement | null>(null)
const mode = ref('')
const { init, write, fit, onData } = useTerminal()

function buildWsUrl(id: string): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${location.host}/ws/${id}`
}

const { connected, send } = useWebSocket(buildWsUrl(sessionId), (msg: WSMessage) => {
  if (msg.type === 'output') {
    write(msg.data)
  } else if (msg.type === 'exit') {
    // Write a styled banner directly into the terminal so the user can see it
    // in context (scroll position, surrounding output, etc.).
    write('\r\n\x1b[2m\x1b[33m── process exited ──\x1b[0m\r\n')
  }
})

function handleResize() {
  const size = fit()
  if (size) send({ type: 'resize', ...size })
}

async function downloadCast() {
  const res = await fetch(`/api/v1/sessions/${sessionId}/cast`)
  const blob = await res.blob()
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `${sessionId}.cast`
  a.click()
}

onMounted(async () => {
  if (!termEl.value) return
  init(termEl.value)
  onData?.((data: string) => send({ type: 'input', data }))
  handleResize()

  // Fetch session info for mode display
  try {
    const res = await fetch(`/api/v1/sessions/${sessionId}`)
    const sess = await res.json()
    mode.value = sess.mode
  } catch { /* ignore */ }

  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.terminal-view {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: #1e1e2e;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.5rem 1rem;
  background: #313244;
  border-bottom: 1px solid #45475a;
  flex-shrink: 0;
}

.session-info {
  color: #a6adc8;
  font-size: 0.85rem;
  flex: 1;
}

.status {
  font-size: 0.8rem;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
}

.connected { background: #a6e3a1; color: #1e1e2e; }
.disconnected { background: #f38ba8; color: #1e1e2e; }

.terminal-container {
  flex: 1;
  overflow: hidden;
  padding: 4px;
}

.btn {
  padding: 0.3rem 0.7rem;
  border: none;
  border-radius: 4px;
  background: #cba6f7;
  color: #1e1e2e;
  cursor: pointer;
  font-size: 0.8rem;
}

.btn-back { background: #45475a; color: #cdd6f4; }
.btn-sm { padding: 0.2rem 0.5rem; }
</style>
