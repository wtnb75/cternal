import { ref, onUnmounted } from 'vue'
import type { WSMessage, ClientMessage } from '@/types'

const RECONNECT_DELAY = 2000
const MAX_RECONNECT = 5

export function useWebSocket(url: string, onMessage: (msg: WSMessage) => void) {
  const connected = ref(false)
  let ws: WebSocket | null = null
  let reconnectCount = 0
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let stopped = false

  function connect() {
    if (stopped) return
    ws = new WebSocket(url)

    ws.onopen = () => {
      connected.value = true
      reconnectCount = 0
    }

    ws.onmessage = (ev: MessageEvent) => {
      try {
        const msg: WSMessage = JSON.parse(ev.data as string)
        onMessage(msg)
      } catch {
        // ignore malformed messages
      }
    }

    ws.onclose = () => {
      connected.value = false
      if (!stopped && reconnectCount < MAX_RECONNECT) {
        reconnectCount++
        reconnectTimer = setTimeout(connect, RECONNECT_DELAY)
      }
    }

    ws.onerror = () => {
      ws?.close()
    }
  }

  function send(msg: ClientMessage) {
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(msg))
    }
  }

  function disconnect() {
    stopped = true
    if (reconnectTimer) clearTimeout(reconnectTimer)
    ws?.close()
    connected.value = false
  }

  connect()
  onUnmounted(disconnect)

  return { connected, send, disconnect }
}
