import { ref, onUnmounted } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { SearchAddon } from '@xterm/addon-search'
import type { ITheme } from '@xterm/xterm'
import '@xterm/xterm/css/xterm.css'

const THEMES: Record<'dark' | 'light', ITheme> = {
  dark: {
    background:          '#1e1e2e',
    foreground:          '#cdd6f4',
    cursor:              '#f5e0dc',
    selectionBackground: 'rgba(205,214,244,0.2)',
    black:        '#45475a', red:     '#f38ba8', green:   '#a6e3a1', yellow:  '#f9e2af',
    blue:         '#89b4fa', magenta: '#f5c2e7', cyan:    '#94e2d5', white:   '#bac2de',
    brightBlack:  '#585b70', brightRed:     '#f38ba8', brightGreen:   '#a6e3a1',
    brightYellow: '#f9e2af', brightBlue:    '#89b4fa', brightMagenta: '#f5c2e7',
    brightCyan:   '#94e2d5', brightWhite:   '#a6adc8',
  },
  light: {
    background:          '#eff1f5',
    foreground:          '#4c4f69',
    cursor:              '#dc8a78',
    selectionBackground: 'rgba(76,79,105,0.2)',
    black:        '#5c5f77', red:     '#d20f39', green:   '#40a02b', yellow:  '#df8e1d',
    blue:         '#1e66f5', magenta: '#ea76cb', cyan:    '#179299', white:   '#acb0be',
    brightBlack:  '#6c6f85', brightRed:     '#d20f39', brightGreen:   '#40a02b',
    brightYellow: '#df8e1d', brightBlue:    '#1e66f5', brightMagenta: '#ea76cb',
    brightCyan:   '#179299', brightWhite:   '#bcc0cc',
  },
}

export function useTerminal() {
  const termRef = ref<HTMLElement | null>(null)
  let terminal: Terminal | null = null
  let fitAddon: FitAddon | null = null
  let searchAddon: SearchAddon | null = null

  function init(el: HTMLElement, scrollback?: number, fontSize?: number, theme: 'dark' | 'light' = 'dark') {
    terminal = new Terminal({
      cursorBlink: true,
      scrollback: scrollback ?? 5000,
      fontSize: fontSize ?? 14,
      theme: THEMES[theme],
    })
    fitAddon = new FitAddon()
    searchAddon = new SearchAddon()
    terminal.loadAddon(fitAddon)
    terminal.loadAddon(searchAddon)
    terminal.open(el)
    fitAddon.fit()
    terminal.focus()
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

  function searchPrev(query: string) {
    searchAddon?.findPrevious(query)
  }

  function setFontSize(size: number) {
    if (!terminal) return
    terminal.options.fontSize = size
    fitAddon?.fit()
  }

  function setTheme(theme: 'dark' | 'light') {
    if (!terminal) return
    terminal.options.theme = THEMES[theme]
  }

  function dispose() {
    terminal?.dispose()
    terminal = null
  }

  onUnmounted(dispose)

  return { termRef, init, write, fit, onData, search, searchPrev, setFontSize, setTheme, dispose, terminal: () => terminal }
}
