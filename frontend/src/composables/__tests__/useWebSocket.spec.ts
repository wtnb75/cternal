import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useWebSocket } from '../useWebSocket'

class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSED = 3
  static instances: MockWebSocket[] = []

  readyState = MockWebSocket.CONNECTING
  onopen: ((e: Event) => void) | null = null
  onmessage: ((e: MessageEvent) => void) | null = null
  onclose: ((e: CloseEvent) => void) | null = null
  onerror: ((e: Event) => void) | null = null
  sent: string[] = []

  constructor(public url: string) {
    MockWebSocket.instances.push(this)
  }

  send(data: string) {
    this.sent.push(data)
  }

  close() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.(new CloseEvent('close'))
  }

  // helpers for tests
  simulateOpen() {
    this.readyState = MockWebSocket.OPEN
    this.onopen?.(new Event('open'))
  }

  simulateMessage(data: string) {
    this.onmessage?.(new MessageEvent('message', { data }))
  }

  simulateError() {
    this.onerror?.(new Event('error'))
  }
}

describe('useWebSocket', () => {
  beforeEach(() => {
    MockWebSocket.instances = []
    vi.stubGlobal('WebSocket', MockWebSocket)
    vi.useFakeTimers()
    vi.spyOn(console, 'warn').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('opens a WebSocket connection immediately', () => {
    useWebSocket('ws://test', () => {})
    expect(MockWebSocket.instances).toHaveLength(1)
    expect(MockWebSocket.instances[0].url).toBe('ws://test')
  })

  it('connected is false before the socket opens', () => {
    const { connected } = useWebSocket('ws://test', () => {})
    expect(connected.value).toBe(false)
  })

  it('connected becomes true on open', () => {
    const { connected } = useWebSocket('ws://test', () => {})
    MockWebSocket.instances[0].simulateOpen()
    expect(connected.value).toBe(true)
  })

  it('connected becomes false on close', () => {
    const { connected } = useWebSocket('ws://test', () => {})
    MockWebSocket.instances[0].simulateOpen()
    MockWebSocket.instances[0].close()
    expect(connected.value).toBe(false)
  })

  it('delivers parsed JSON messages to the callback', () => {
    const received: unknown[] = []
    useWebSocket('ws://test', msg => received.push(msg))
    MockWebSocket.instances[0].simulateMessage('{"type":"output","data":"hello"}')
    expect(received).toEqual([{ type: 'output', data: 'hello' }])
  })

  it('silently ignores malformed JSON messages', () => {
    const received: unknown[] = []
    useWebSocket('ws://test', msg => received.push(msg))
    MockWebSocket.instances[0].simulateMessage('not-json{{')
    expect(received).toHaveLength(0)
  })

  it('reconnects 2 seconds after close', () => {
    useWebSocket('ws://test', () => {})
    MockWebSocket.instances[0].simulateOpen()
    MockWebSocket.instances[0].close()

    vi.advanceTimersByTime(1999)
    expect(MockWebSocket.instances).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(MockWebSocket.instances).toHaveLength(2)
  })

  it('resets reconnect count after a successful open', () => {
    useWebSocket('ws://test', () => {})
    // close and reconnect once
    MockWebSocket.instances[0].simulateOpen()
    MockWebSocket.instances[0].close()
    vi.advanceTimersByTime(2000)
    // open the reconnected socket → count resets
    MockWebSocket.instances[1].simulateOpen()
    MockWebSocket.instances[1].close()
    vi.advanceTimersByTime(2000)
    expect(MockWebSocket.instances).toHaveLength(3)
  })

  it('stops reconnecting after 5 attempts', () => {
    useWebSocket('ws://test', () => {})
    // Close without opening so reconnectCount accumulates without resetting.
    for (let i = 0; i < 5; i++) {
      MockWebSocket.instances[i].close()
      vi.advanceTimersByTime(2000)
    }
    // initial + 5 reconnects = 6 total instances
    expect(MockWebSocket.instances).toHaveLength(6)
    // 6th close: reconnectCount === 5, 5 < 5 is false → no more reconnects
    MockWebSocket.instances[5].close()
    vi.advanceTimersByTime(2000)
    expect(MockWebSocket.instances).toHaveLength(6)
  })

  it('disconnect prevents reconnection', () => {
    const { disconnect } = useWebSocket('ws://test', () => {})
    MockWebSocket.instances[0].simulateOpen()
    disconnect()
    vi.advanceTimersByTime(5000)
    expect(MockWebSocket.instances).toHaveLength(1)
  })

  it('disconnect sets connected to false', () => {
    const { connected, disconnect } = useWebSocket('ws://test', () => {})
    MockWebSocket.instances[0].simulateOpen()
    disconnect()
    expect(connected.value).toBe(false)
  })

  it('send serializes and transmits the message when OPEN', () => {
    const { send } = useWebSocket('ws://test', () => {})
    MockWebSocket.instances[0].simulateOpen()
    send({ type: 'input', data: 'hello' })
    expect(MockWebSocket.instances[0].sent).toEqual(['{"type":"input","data":"hello"}'])
  })

  it('send does nothing when socket is not OPEN', () => {
    const { send } = useWebSocket('ws://test', () => {})
    // readyState is CONNECTING — not OPEN
    send({ type: 'ping' })
    expect(MockWebSocket.instances[0].sent).toHaveLength(0)
  })

  it('an error triggers close and then reconnect', () => {
    useWebSocket('ws://test', () => {})
    MockWebSocket.instances[0].simulateOpen()
    MockWebSocket.instances[0].simulateError()
    vi.advanceTimersByTime(2000)
    expect(MockWebSocket.instances).toHaveLength(2)
  })
})
