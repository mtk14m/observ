import { afterEach, describe, expect, test, vi } from 'vitest'
import { ApiError, api } from './api'

function mockFetch(status: number, body: unknown) {
  const fn = vi.fn().mockResolvedValue(new Response(JSON.stringify(body), { status }))
  vi.stubGlobal('fetch', fn)
  return fn
}

afterEach(() => vi.unstubAllGlobals())

describe('api', () => {
  test('passes the time range and parameters and unwraps data', async () => {
    const fetch = mockFetch(200, { data: [{ name: 'api' }] })
    const got = await api.services({ from: 'now-1h', to: 'now' })
    expect(got).toEqual([{ name: 'api' }])
    expect(fetch.mock.calls[0]![0]).toBe('/api/v1/services?from=now-1h&to=now')
  })

  test('omits empty parameters and repeats array ones', async () => {
    const fetch = mockFetch(200, { data: [] })
    await api.queryMetric({ from: 'now-1h', to: 'now' }, {
      metric: 'm', agg: '', groupBy: ['a', 'b'], filters: ['k:v', 'x:y'],
    })
    expect(fetch.mock.calls[0]![0]).toBe(
      '/api/v1/metrics/query?from=now-1h&to=now&metric=m&group_by=a%2Cb&filter=k%3Av&filter=x%3Ay',
    )
  })

  test('encodes trace IDs in the path', async () => {
    const fetch = mockFetch(200, { data: [] })
    await api.trace('abc/../x')
    expect(fetch.mock.calls[0]![0]).toBe('/api/v1/traces/abc%2F..%2Fx')
  })

  test('turns error responses into ApiError', async () => {
    mockFetch(400, { error: 'logsearch: syntax error' })
    const err = await api.logs({ from: 'now-1h', to: 'now' }, '"bad').catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).message).toBe('logsearch: syntax error')
    expect((err as ApiError).status).toBe(400)
  })
})

describe('write requests', () => {
  test('send JSON and unwrap data', async () => {
    const fetch = mockFetch(201, { data: { id: 'r1' } })
    const rule = { name: 'x', kind: 'logs' as const, query: '', op: '>' as const, threshold: 1,
      window_seconds: 300, for_seconds: 0, channels: [], enabled: true }
    expect(await api.createRule(rule)).toEqual({ id: 'r1' })
    const [url, init] = fetch.mock.calls[0]!
    expect(url).toBe('/api/v1/alerts/rules')
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body)).toEqual(rule)
  })

  test('handle empty 204 responses and errors', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 204 })))
    await expect(api.deleteRule('r1')).resolves.toBeUndefined()
    mockFetch(409, { error: 'in use' })
    await expect(api.deleteChannel('c1')).rejects.toMatchObject({ message: 'in use', status: 409 })
  })
})
