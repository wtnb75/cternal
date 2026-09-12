export interface TerminalFontOption {
  label: string
  value: string
}

export const TERMINAL_FONTS: TerminalFontOption[] = [
  { label: 'System Default', value: 'Menlo, Consolas, monospace' },
  { label: 'Menlo', value: 'Menlo, monospace' },
  { label: 'Consolas', value: 'Consolas, monospace' },
  { label: 'SF Mono', value: '"SF Mono", Menlo, monospace' },
  { label: 'Cascadia Mono', value: '"Cascadia Mono", monospace' },
  { label: 'JetBrains Mono', value: '"JetBrains Mono", monospace' },
  { label: 'Fira Code', value: '"Fira Code", monospace' },
  { label: 'Source Code Pro', value: '"Source Code Pro", monospace' },
  { label: 'DejaVu Sans Mono', value: '"DejaVu Sans Mono", monospace' },
]

export const DEFAULT_TERMINAL_FONT: string = TERMINAL_FONTS[0]!.value
