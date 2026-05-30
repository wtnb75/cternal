declare global {
  interface Window {
    __BASE_PATH__?: string
  }
}

export function apiUrl(path: string): string {
  return (window.__BASE_PATH__ ?? '') + path
}

export function wsUrl(id: string): string {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${location.host}${window.__BASE_PATH__ ?? ''}/ws/${id}`
}
