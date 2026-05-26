export interface Container {
  id: string
  name: string
  image: string
  status: string
  running: boolean
}

export interface Session {
  id: string
  containerId: string
  mode: 'exec' | 'attach' | 'logs'
  status: 'active' | 'disconnected'
  wsUrl: string
}

export interface CreateSessionRequest {
  containerId: string
  mode: 'exec' | 'attach' | 'logs'
  shell?: string[]
  since?: string
  cols?: number
  rows?: number
}

export type WSMessage =
  | { type: 'output'; data: string }
  | { type: 'error'; message: string }
  | { type: 'pong' }

export type ClientMessage =
  | { type: 'input'; data: string }
  | { type: 'resize'; cols: number; rows: number }
  | { type: 'ping' }
