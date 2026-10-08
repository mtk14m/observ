import { describe, expect, test } from 'vitest'
import { serviceHealth, issueLinks, longestLiteral } from './health'
import type { Issue, ServiceSummary } from './api'

const svc = (name: string, o: Partial<ServiceSummary> = {}): ServiceSummary => ({
  name, requests: 1000, errors: 0, error_rate: 0, rate_per_second: 1, p50_ms: 20, p95_ms: 100, p99_ms: 200, ...o,
})

describe('serviceHealth', () => {
  test('classifies services and explains why, worst first', () => {
    const now = [
      svc('inventory'),
      svc('checkout', { error_rate: 0.11, errors: 110 }),
      svc('payment', { p95_ms: 840 }),
      svc('frontend', { error_rate: 0.02, errors: 20 }),
    ]
    const before = [svc('inventory'), svc('checkout', { error_rate: 0.1 }), svc('payment', { p95_ms: 300 }), svc('frontend')]
    const h = serviceHealth(now, before)
    expect(h.map((s) => [s.name, s.status])).toEqual([
      ['checkout', 'critical'], ['payment', 'critical'], ['frontend', 'warning'], ['inventory', 'ok'],
    ])
    expect(h[0]!.reasons).toEqual(['11% errors'])
    expect(h[1]!.reasons).toEqual(['p95 ×2.8 vs previous period'])
    expect(h[2]!.reasons).toEqual(['2% errors'])
    expect(h[3]!.reasons).toEqual([])
  })

  test('services without a previous period are judged on errors only', () => {
    const h = serviceHealth([svc('new', { p95_ms: 5000 })], [])
    expect(h[0]!.status).toBe('ok')
  })

  test('small latencies do not raise alarms', () => {
    const h = serviceHealth([svc('fast', { p95_ms: 30 })], [svc('fast', { p95_ms: 10 })])
    expect(h[0]!.status).toBe('ok')
  })
})

describe('issues', () => {
  test('longestLiteral keeps the most specific fixed part of a template', () => {
    expect(longestLiteral('order <*> refused by issuer <*>')).toBe('refused by issuer')
    expect(longestLiteral('<*>')).toBe('')
  })

  test('links lead to the logs or traces of the issue', () => {
    const base: Issue = {
      id: 'i', service: 'payment', kind: 'log', title: 'payment <*> refused', example: '', count: 1, previous_count: 0,
      first_seen: 0, last_seen: 0, status: 'new', buckets: [], example_trace_id: 't1',
    }
    expect(issueLinks(base)).toEqual({
      logs: { path: '/logs', query: { q: 'service:payment level:error "refused"' } },
      traces: { path: '/traces', query: { q: 'service:payment status:error' } },
      trace: { path: '/traces/t1' },
    })
    expect(issueLinks({ ...base, kind: 'span', title: 'charge card: card declined', example_trace_id: '' }).trace).toBeUndefined()
  })
})
