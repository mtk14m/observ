/**
 * Time ranges are kept as expressions ("now-1h", "now", or an ISO date) so
 * they can live in the URL and stay relative until a query is run.
 */
export interface TimeRange {
  from: string
  to: string
}

export interface Preset {
  from: string
  label: string
}

export const PRESETS: readonly Preset[] = [
  { from: 'now-15m', label: 'Past 15 minutes' },
  { from: 'now-30m', label: 'Past 30 minutes' },
  { from: 'now-1h', label: 'Past 1 hour' },
  { from: 'now-4h', label: 'Past 4 hours' },
  { from: 'now-1d', label: 'Past 1 day' },
  { from: 'now-2d', label: 'Past 2 days' },
  { from: 'now-7d', label: 'Past 7 days' },
  { from: 'now-30d', label: 'Past 30 days' },
]

export const DEFAULT_RANGE: TimeRange = { from: 'now-1h', to: 'now' }

const UNIT_MS: Record<string, number> = {
  s: 1_000,
  m: 60_000,
  h: 3_600_000,
  d: 86_400_000,
  w: 604_800_000,
}

const RELATIVE = /^now(?:-(\d+)([smhdw]))?$/
const ISO = /^\d{4}-\d{2}-\d{2}T/

/** Parses a time expression relative to `now`. Returns null when invalid. */
export function parseTime(expr: string, now: Date): Date | null {
  const m = RELATIVE.exec(expr)
  if (m) {
    const [, amount, unit] = m
    if (amount === undefined || unit === undefined) return new Date(now)
    return new Date(now.getTime() - Number(amount) * UNIT_MS[unit]!)
  }
  if (ISO.test(expr)) {
    const d = new Date(expr)
    return Number.isNaN(d.getTime()) ? null : d
  }
  return null
}

/** Resolves a range to absolute dates, or null if invalid or empty. */
export function resolve(range: TimeRange, now: Date): { from: Date; to: Date } | null {
  const from = parseTime(range.from, now)
  const to = parseTime(range.to, now)
  if (!from || !to || from >= to) return null
  return { from, to }
}

/** Returns a human-readable label for the range. */
export function label(range: TimeRange): string {
  if (range.to === 'now') {
    const preset = PRESETS.find((p) => p.from === range.from)
    if (preset) return preset.label
  }
  return `${range.from} → ${range.to}`
}
