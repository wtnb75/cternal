import { ref, onUnmounted } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { SearchAddon } from '@xterm/addon-search'

export function useTerminal() {
  const termRef = ref<HTMLElement | null>(null)
  let terminal: Terminal | null = null
  let fitAddon: FitAddon | null = null
  let searchAddon: SearchAddon | null = null

  function init(el: HTMLElement) {
    terminal = new Terminal({
      cursorBlink: true,
      scrollback: 5000,
      theme: { background: '#1e1e2e' },
    })
    fitAddon = new FitAddon()
    searchAddon = new SearchAddon()
    terminal.loadAddon(fitAddon)
    terminal.loadAddon(searchAddon)
    terminal.open(el)
    fitAddon.fit()
    termRef.value = el
  }

  function write(data: string) {
    terminal?.write(data)
  }

  function fit(): { cols: number; rows: number } | null {
    if (!fitAddon || !terminal) return null
    fitAddon.fit()
    return { cols: terminal.cols, rows: terminal.rows }
  }

  function onData(handler: (data: string) => void) {
    return terminal?.onData(handler)
  }

  function search(query: string) {
    searchAddon?.findNext(query)
  }

  function dispose() {
    terminal?.dispose()
    terminal = null
  }

  onUnmounted(dispose)

  return { termRef, init, write, fit, onData, search, dispose, terminal: () => terminal }
}
