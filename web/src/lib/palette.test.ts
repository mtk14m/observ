import { describe, expect, test } from 'vitest'
import { paletteItems, shiftRange } from './palette'

const sources = { services: ['checkout', 'payment'], metrics: ['http.server.request.duration', 'shop.orders'] }

describe('paletteItems', () => {
  test('without input, lists the pages', () => {
    const items = paletteItems('', sources)
    expect(items.slice(0, 3).map((i) => i.label)).toEqual(['Home', 'Services', 'Issues'])
  })

  test('matches pages, services and metrics', () => {
    const items = paletteItems('pay', sources)
    expect(items.map((i) => i.label)).toEqual([
      'Service payment', 'Search logs for “pay”', 'Search traces for “pay”',
    ])
    expect(items[0]!.to).toEqual({ path: '/services/payment' })
    expect(paletteItems('duration', sources)[0]).toMatchObject({
      label: 'Metric http.server.request.duration',
      to: { path: '/metrics', query: { metric: 'http.server.request.duration' } },
    })
  })

  test('a pasted trace ID opens the trace', () => {
    const id = '0af7651916cd43dd8448eb211c80319c'
    expect(paletteItems(` ${id} `, sources)[0]).toEqual({ label: `Open trace ${id}`, to: { path: `/traces/${id}` } })
  })
})

describe('shiftRange', () => {
  const now = new Date('2026-10-07T12:00:00Z')

  test('moves a relative range back and forth by its length', () => {
    expect(shiftRange({ from: 'now-1h', to: 'now' }, -1, now)).toEqual({
      from: '2026-10-07T10:00:00.000Z', to: '2026-10-07T11:00:00.000Z',
    })
    expect(shiftRange({ from: '2026-10-07T10:00:00.000Z', to: '2026-10-07T11:00:00.000Z' }, 1, now)).toEqual({
      from: '2026-10-07T11:00:00.000Z', to: '2026-10-07T12:00:00.000Z',
    })
  })

  test('never goes past now', () => {
    expect(shiftRange({ from: 'now-1h', to: 'now' }, 1, now)).toBeNull()
  })
})
