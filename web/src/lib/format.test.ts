import { describe, expect, test } from 'vitest'
import { formatCompact, formatDuration, formatMs, formatPercent, formatRate, formatTime } from './format'

describe('formatDuration (nanoseconds)', () => {
  test.each([
    [0, '0 ms'],
    [850, '850 ns'],
    [42_000, '42 µs'],
    [1_500_000, '1.5 ms'],
    [12_345_678, '12.3 ms'],
    [250_000_000, '250 ms'],
    [1_250_000_000, '1.25 s'],
    [95_000_000_000, '1m 35s'],
  ])('%d → %s', (ns, want) => expect(formatDuration(ns)).toBe(want))
})

test('formatMs', () => {
  expect(formatMs(0.42)).toBe('420 µs')
  expect(formatMs(190.5)).toBe('191 ms')
})

test('formatTime shows local time with milliseconds', () => {
  const ns = new Date(2026, 9, 7, 13, 2, 11, 123).getTime() * 1e6
  expect(formatTime(ns)).toBe('13:02:11.123')
})

test('formatPercent', () => {
  expect(formatPercent(0)).toBe('0%')
  expect(formatPercent(0.0834)).toBe('8.3%')
  expect(formatPercent(1)).toBe('100%')
})

test('formatRate', () => {
  expect(formatRate(0.25)).toBe('0.25/s')
  expect(formatRate(12.345)).toBe('12.3/s')
})

test('formatCompact', () => {
  expect(formatCompact(950)).toBe('950')
  expect(formatCompact(12_300)).toBe('12.3K')
  expect(formatCompact(4_200_000)).toBe('4.2M')
  expect(formatCompact(0.125)).toBe('0.125')
})
