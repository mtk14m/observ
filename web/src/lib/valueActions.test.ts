import { describe, expect, test } from 'vitest'
import { valueActions } from './valueActions'

const range = { from: 'now-30m', to: 'now' }

function labels(actions: ReturnType<typeof valueActions>) {
  return actions.map((a) => a.label)
}

describe('valueActions', () => {
  test('in the logs explorer, filtering edits the current search', () => {
    const actions = valueActions({ key: 'payment.issuer', value: 'acme-bank' }, { path: '/logs', query: { ...range, q: 'level:error' } })
    expect(labels(actions)).toEqual(['Filter on this value', 'Exclude this value', 'View traces with this value', 'Copy value'])
    expect(actions[0]!.to).toEqual({ path: '/logs', query: { ...range, q: 'level:error payment.issuer:acme-bank' } })
    expect(actions[1]!.to).toEqual({ path: '/logs', query: { ...range, q: 'level:error -payment.issuer:acme-bank' } })
    expect(actions[2]!.to).toEqual({ path: '/traces', query: { ...range, q: 'payment.issuer:acme-bank' } })
    expect(actions[3]!.copy).toBe('acme-bank')
  })

  test('elsewhere, filtering opens the logs explorer', () => {
    const actions = valueActions({ key: 'http.route', value: 'POST /pay' }, { path: '/services/checkout', query: range })
    expect(labels(actions)).toContain('View logs with this value')
    const logs = actions.find((a) => a.label === 'View logs with this value')!
    expect(logs.to).toEqual({ path: '/logs', query: { ...range, q: 'http.route:"POST /pay"' } })
  })

  test('services open their page and use the short query key', () => {
    const actions = valueActions({ key: 'service.name', value: 'payment' }, { path: '/traces', query: range })
    expect(labels(actions)[0]).toBe('Open service payment')
    expect(actions[0]!.to).toEqual({ path: '/services/payment', query: range })
    expect(actions.find((a) => a.label === 'Filter on this value')!.to)
      .toEqual({ path: '/traces', query: { ...range, q: 'service:payment' } })
  })

  test('trace IDs open the trace', () => {
    const actions = valueActions({ key: 'trace_id', value: 'abc' }, { path: '/logs', query: range })
    expect(actions[0]).toMatchObject({ label: 'Open trace', to: { path: '/traces/abc' } })
  })

  test('log levels filter logs but are not offered for traces', () => {
    const actions = valueActions({ key: 'level', value: 'ERROR' }, { path: '/logs', query: range })
    expect(actions[0]!.to).toEqual({ path: '/logs', query: { ...range, q: 'level:error' } })
    expect(labels(actions)).not.toContain('View traces with this value')
  })
})
