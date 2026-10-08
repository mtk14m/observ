import type { RouteLocationRaw } from 'vue-router'
import { resolve, type TimeRange } from './timeRange'

export interface PaletteItem {
  label: string
  to: RouteLocationRaw
}

const PAGES: PaletteItem[] = [
  { label: 'Home', to: { path: '/' } },
  { label: 'Services', to: { path: '/services' } },
  { label: 'Issues', to: { path: '/issues' } },
  { label: 'Traces explorer', to: { path: '/traces' } },
  { label: 'Logs explorer', to: { path: '/logs' } },
  { label: 'Metrics explorer', to: { path: '/metrics' } },
  { label: 'Alerts', to: { path: '/alerts' } },
  { label: 'Settings', to: { path: '/settings' } },
]

const TRACE_ID = /^[0-9a-f]{32}$/i

/** Items of the command palette (Cmd+K) for what the user typed. */
export function paletteItems(input: string, sources: { services: string[]; metrics: string[] }): PaletteItem[] {
  const text = input.trim()
  const needle = text.toLowerCase()
  if (!needle) return PAGES
  if (TRACE_ID.test(text)) return [{ label: `Open trace ${text.toLowerCase()}`, to: { path: `/traces/${text.toLowerCase()}` } }]

  const match = (s: string) => s.toLowerCase().includes(needle)
  return [
    ...PAGES.filter((p) => match(p.label)),
    ...sources.services.filter(match).map((s) => ({ label: `Service ${s}`, to: { path: `/services/${s}` } })),
    ...sources.metrics.filter(match).map((m) => ({ label: `Metric ${m}`, to: { path: '/metrics', query: { metric: m } } })),
    { label: `Search logs for “${text}”`, to: { path: '/logs', query: { q: text } } },
    { label: `Search traces for “${text}”`, to: { path: '/traces', query: { q: text } } },
  ]
}

/**
 * Moves a range by its own length, backwards (-1) or forwards (1). Returns
 * null when the move would go past now.
 */
export function shiftRange(range: TimeRange, direction: -1 | 1, now: Date): TimeRange | null {
  const r = resolve(range, now)
  if (!r) return null
  const len = r.to.getTime() - r.from.getTime()
  const from = r.from.getTime() + direction * len
  const to = r.to.getTime() + direction * len
  if (to > now.getTime()) return null
  return { from: new Date(from).toISOString(), to: new Date(to).toISOString() }
}
