export interface Container {
  id: string
  name: string
  image: string
  status: string
  running: boolean
  runtime?: string
  labels?: Record<string, string>
}

export interface Session {
  id: string
  containerId: string
  containerName?: string
  runtime?: string
  mode: 'exec' | 'attach' | 'logs'
  status: 'active' | 'disconnected'
  wsUrl: string
  createdAt?: string
  cols?: number
  rows?: number
}

export interface CreateSessionRequest {
  containerId: string
  containerName?: string
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
  | { type: 'exit' }

export type ClientMessage =
  | { type: 'input'; data: string }
  | { type: 'resize'; cols: number; rows: number }
  | { type: 'ping' }
