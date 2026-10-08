import type { RouteLocationRaw } from 'vue-router'
import type { Issue, ServiceSummary } from './api'
import { formatPercent } from './format'

export type Health = 'critical' | 'warning' | 'ok'

export interface ServiceHealth extends ServiceSummary {
  status: Health
  /** Why the service is not ok, in plain words. */
  reasons: string[]
  /** p95 latency relative to the previous period (1 = unchanged), if known. */
  p95Ratio?: number
}

const RANK: Record<Health, number> = { critical: 0, warning: 1, ok: 2 }
const MIN_SLOW_MS = 200 // below this, latency changes are not worth a signal

/**
 * A traffic light per service: errors and latency compared with the
 * previous period of the same length. Worst services first.
 */
export function serviceHealth(now: ServiceSummary[], before: ServiceSummary[]): ServiceHealth[] {
  const prev = new Map(before.map((s) => [s.name, s]))
  return now
    .map((s) => {
      const reasons: string[] = []
      let status: Health = 'ok'
      const worse = (h: Health) => {
        if (RANK[h] < RANK[status]) status = h
      }
      if (s.error_rate >= 0.05) {
        worse('critical')
        reasons.push(`${formatPercent(s.error_rate)} errors`)
      } else if (s.error_rate >= 0.01) {
        worse('warning')
        reasons.push(`${formatPercent(s.error_rate)} errors`)
      }
      const p = prev.get(s.name)
      const p95Ratio = p && p.p95_ms > 0 ? s.p95_ms / p.p95_ms : undefined
      if (p95Ratio !== undefined && s.p95_ms >= MIN_SLOW_MS && p95Ratio >= 1.5) {
        worse(p95Ratio >= 2 ? 'critical' : 'warning')
        reasons.push(`p95 ×${Number(p95Ratio.toFixed(1))} vs previous period`)
      }
      return { ...s, status, reasons, p95Ratio }
    })
    .sort((a, b) => RANK[a.status] - RANK[b.status] || b.error_rate - a.error_rate || a.name.localeCompare(b.name))
}

/**
 * The longest fixed part of an issue template, to search for it. On ties
 * the later part wins: messages usually end with the specific detail.
 */
export function longestLiteral(template: string): string {
  return template
    .split('<*>')
    .map((s) => s.trim())
    .reduce((a, b) => (b.length >= a.length && b ? b : a), '')
}

/** Where to look at an issue's records. */
export function issueLinks(issue: Issue): { logs: RouteLocationRaw; traces: RouteLocationRaw; trace?: RouteLocationRaw } {
  const literal = issue.kind === 'log' ? longestLiteral(issue.title) : ''
  const logsQ = `service:${issue.service} level:error${literal ? ` "${literal}"` : ''}`
  return {
    logs: { path: '/logs', query: { q: logsQ } },
    traces: { path: '/traces', query: { q: `service:${issue.service} status:error` } },
    ...(issue.example_trace_id ? { trace: { path: `/traces/${issue.example_trace_id}` } } : {}),
  }
}
