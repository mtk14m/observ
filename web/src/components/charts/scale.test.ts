import { expect, test } from 'vitest'
import { niceMax, timeTicks } from './scale'

test('niceMax rounds up to readable values', () => {
  expect(niceMax(0)).toBe(1)
  expect(niceMax(7)).toBe(10)
  expect(niceMax(180)).toBe(200)
  expect(niceMax(0.42)).toBe(0.5)
})

test('timeTicks lands on round boundaries', () => {
  const from = Date.UTC(2026, 9, 7, 12, 3) * 1e6
  const to = Date.UTC(2026, 9, 7, 13, 3) * 1e6
  const ticks = timeTicks(from, to, 6)
  expect(ticks.length).toBeGreaterThanOrEqual(4)
  expect(ticks.length).toBeLessThanOrEqual(7)
  for (const t of ticks) expect((t / 1e6) % (10 * 60_000)).toBe(0)
})
