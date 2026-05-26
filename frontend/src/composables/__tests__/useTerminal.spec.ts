import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { SearchAddon } from '@xterm/addon-search'
import { useTerminal } from '../useTerminal'

// vi.hoisted runs before vi.mock factories, so these objects are safe to
// reference inside the factory closures below.
const { mockTerm, mockFit, mockSearch } = vi.hoisted(() => ({
  mockTerm: {
    loadAddon: vi.fn(),
    open: vi.fn(),
    write: vi.fn(),
    dispose: vi.fn(),
    onData: vi.fn(),
    cols: 80,
    rows: 24,
  },
  mockFit: { fit: vi.fn() },
  mockSearch: { findNext: vi.fn() },
}))

// Use regular functions (not arrow) in vi.fn() so they are valid constructors.
vi.mock('@xterm/xterm', () => ({ Terminal: vi.fn(function () { return mockTerm }) }))
vi.mock('@xterm/addon-fit', () => ({ FitAddon: vi.fn(function () { return mockFit }) }))
vi.mock('@xterm/addon-search', () => ({ SearchAddon: vi.fn(function () { return mockSearch }) }))

describe('useTerminal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockTerm.onData.mockReturnValue({ dispose: vi.fn() })
    vi.spyOn(console, 'warn').mockImplementation(() => {})
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('init creates a Terminal and opens it on the element', () => {
    const { init } = useTerminal()
    const el = document.createElement('div')
    init(el)
    expect(Terminal).toHaveBeenCalledOnce()
    expect(mockTerm.open).toHaveBeenCalledWith(el)
  })

  it('init loads both FitAddon and SearchAddon', () => {
    const { init } = useTerminal()
    init(document.createElement('div'))
    expect(mockTerm.loadAddon).toHaveBeenCalledTimes(2)
  })

  it('init calls fit immediately after opening', () => {
    const { init } = useTerminal()
    init(document.createElement('div'))
    expect(mockFit.fit).toHaveBeenCalled()
  })

  it('write passes data to terminal.write', () => {
    const { init, write } = useTerminal()
    init(document.createElement('div'))
    write('hello world')
    expect(mockTerm.write).toHaveBeenCalledWith('hello world')
  })

  it('write is a no-op before init', () => {
    const { write } = useTerminal()
    expect(() => write('hello')).not.toThrow()
  })

  it('fit calls fitAddon.fit and returns terminal dimensions', () => {
    const { init, fit } = useTerminal()
    init(document.createElement('div'))
    const size = fit()
    expect(size).toEqual({ cols: 80, rows: 24 })
  })

  it('fit returns null before init', () => {
    const { fit } = useTerminal()
    expect(fit()).toBeNull()
  })

  it('onData registers a handler on the terminal', () => {
    const { init, onData } = useTerminal()
    init(document.createElement('div'))
    const handler = vi.fn()
    onData(handler)
    expect(mockTerm.onData).toHaveBeenCalledWith(handler)
  })

  it('search calls findNext on SearchAddon', () => {
    const { init, search } = useTerminal()
    init(document.createElement('div'))
    search('error')
    expect(mockSearch.findNext).toHaveBeenCalledWith('error')
  })

  it('search is a no-op before init', () => {
    const { search } = useTerminal()
    expect(() => search('anything')).not.toThrow()
  })

  it('dispose calls terminal.dispose and clears the instance', () => {
    const { init, dispose, terminal } = useTerminal()
    init(document.createElement('div'))
    expect(terminal()).not.toBeNull()
    dispose()
    expect(mockTerm.dispose).toHaveBeenCalled()
    expect(terminal()).toBeNull()
  })
})
