import { describe, expect, test } from 'vitest'
import { DEFAULT_RANGE, PRESETS, label, parseTime, resolve } from './timeRange'

const now = new Date('2026-10-07T12:00:00Z')

describe('parseTime', () => {
  test.each([
    ['now', '2026-10-07T12:00:00.000Z'],
    ['now-30s', '2026-10-07T11:59:30.000Z'],
    ['now-15m', '2026-10-07T11:45:00.000Z'],
    ['now-4h', '2026-10-07T08:00:00.000Z'],
    ['now-2d', '2026-10-05T12:00:00.000Z'],
    ['now-1w', '2026-09-30T12:00:00.000Z'],
    ['2026-10-01T00:00:00Z', '2026-10-01T00:00:00.000Z'],
  ])('%s', (expr, want) => {
    expect(parseTime(expr, now)?.toISOString()).toBe(want)
  })

  test.each(['', 'yesterday', 'now-', 'now-5', 'now-5y', 'now+1h', 'now-1.5h'])(
    'rejects %j',
    (expr) => {
      expect(parseTime(expr, now)).toBeNull()
    },
  )
})

describe('resolve', () => {
  test('returns absolute bounds', () => {
    const r = resolve({ from: 'now-1h', to: 'now' }, now)
    expect(r).toEqual({ from: new Date('2026-10-07T11:00:00Z'), to: now })
  })

  test('returns null when from is not before to', () => {
    expect(resolve({ from: 'now', to: 'now-1h' }, now)).toBeNull()
  })

  test('returns null on invalid input', () => {
    expect(resolve({ from: 'garbage', to: 'now' }, now)).toBeNull()
  })
})

describe('label', () => {
  test('names presets', () => {
    expect(label({ from: 'now-1h', to: 'now' })).toBe('Past 1 hour')
    expect(label({ from: 'now-7d', to: 'now' })).toBe('Past 7 days')
  })

  test('formats absolute ranges in local time', () => {
    const from = new Date(2026, 9, 7, 21, 2).toISOString()
    const to = new Date(2026, 9, 7, 21, 15, 30).toISOString()
    expect(label({ from, to })).toBe('Oct 7, 21:02 → 21:15')
    const nextDay = new Date(2026, 9, 8, 1, 0).toISOString()
    expect(label({ from, to: nextDay })).toBe('Oct 7, 21:02 → Oct 8, 01:00')
  })

  test('falls back to the raw expressions', () => {
    expect(label({ from: 'now-3h', to: 'now-1h' })).toBe('now-3h → now-1h')
  })
})

test('default range is a preset', () => {
  expect(PRESETS.some((p) => p.from === DEFAULT_RANGE.from)).toBe(true)
  expect(DEFAULT_RANGE.to).toBe('now')
})
