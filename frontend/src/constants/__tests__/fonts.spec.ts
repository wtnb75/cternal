import { describe, it, expect } from 'vitest'
import { TERMINAL_FONTS, DEFAULT_TERMINAL_FONT } from '../fonts'

describe('TERMINAL_FONTS', () => {
  it('contains at least one selectable font', () => {
    expect(TERMINAL_FONTS.length).toBeGreaterThan(0)
  })

  it('every entry has a non-empty label and a monospace fallback in its value', () => {
    for (const font of TERMINAL_FONTS) {
      expect(font.label.length).toBeGreaterThan(0)
      expect(font.value).toMatch(/monospace$/)
    }
  })

  it('has no duplicate values', () => {
    const values = TERMINAL_FONTS.map(f => f.value)
    expect(new Set(values).size).toBe(values.length)
  })
})

describe('DEFAULT_TERMINAL_FONT', () => {
  it('matches the value of the first font entry', () => {
    expect(DEFAULT_TERMINAL_FONT).toBe(TERMINAL_FONTS[0]!.value)
  })
})
