import { expect, test } from 'vitest'
import { aggregations, seriesLabel, valueFormatter } from './metrics'

test('histograms default to p95, gauges to average', () => {
  expect(aggregations({ type: 'Histogram' })[0]!.value).toBe('p95')
  expect(aggregations({ type: 'Gauge' })[0]!.value).toBe('avg')
})

test('counters are shown as a total rate by default', () => {
  const opts = aggregations({ type: 'Sum', monotonic: true })
  expect(opts[0]).toEqual({ value: 'sum', label: 'rate (total)' })
  expect(opts.map((o) => o.value)).toEqual(['sum', 'avg', 'max', 'min'])
})

test('value formatting follows units and rates', () => {
  const m = { name: 'd', type: 'Histogram', unit: 's', series: 1, attribute_keys: [], monotonic: false }
  expect(valueFormatter(m, 'p95', false)(0.25)).toBe('250 ms')
  expect(valueFormatter(m, 'count', false)(12)).toBe('12/s')
  const c = { name: 'o', type: 'Sum', unit: '{order}', series: 1, attribute_keys: [], monotonic: true }
  expect(valueFormatter(c, 'sum', true)(1.5)).toBe('1.5/s')
})

test('series labels', () => {
  expect(seriesLabel({}, 'cpu')).toBe('cpu')
  expect(seriesLabel({ route: '/pay', service: 'api' }, 'x')).toBe('/pay · api')
  expect(seriesLabel({ route: '' }, 'x')).toBe('(none)')
})
