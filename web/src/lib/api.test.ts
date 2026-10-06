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
