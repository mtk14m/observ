/** Log severity levels, from most to least severe, with their color token. */
export const LEVELS = ['FATAL', 'ERROR', 'WARN', 'INFO', 'DEBUG', 'TRACE', 'UNSET'] as const

export function levelColor(level: string): string {
  switch (level.toUpperCase()) {
    case 'FATAL':
    case 'ERROR':
      return 'var(--level-error)'
    case 'WARN':
    case 'WARNING':
      return 'var(--level-warn)'
    case 'INFO':
      return 'var(--level-info)'
    default:
      return 'var(--level-debug)'
  }
}

export function levelRank(level: string): number {
  const i = LEVELS.indexOf(level.toUpperCase() as (typeof LEVELS)[number])
  return i < 0 ? LEVELS.length : i
}
