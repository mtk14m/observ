import { expect, test } from 'vitest'
import { mount } from '@vue/test-utils'
import type { HistogramBucket } from '@/lib/api'
import StackedBars from './StackedBars.vue'

const s = 1e9
const buckets: HistogramBucket[] = [
  { t: 0, counts: { INFO: 10, ERROR: 2 } },
  { t: 10 * s, counts: { INFO: 4 } },
]

test('stacks severities with errors at the bottom and shows a legend', () => {
  const w = mount(StackedBars, { props: { buckets, from: 0, to: 20 * s } })
  const first = w.findAll('g.bar')[0]!.findAll('rect:not(.hit)')
  expect(first).toHaveLength(2)
  // ERROR is drawn first, on the baseline (largest y).
  expect(Number(first[0]!.attributes('y'))).toBeGreaterThan(Number(first[1]!.attributes('y')))
  expect(w.findAll('.legend li').map((l) => l.text())).toEqual(['ERROR', 'INFO'])
})

test('hovering a bar shows its counts', async () => {
  const w = mount(StackedBars, { props: { buckets, from: 0, to: 20 * s } })
  await w.findAll('g.bar')[0]!.trigger('pointerenter')
  expect(w.find('.tooltip').text()).toContain('10INFO')
  expect(w.find('.tooltip').text()).toContain('2ERROR')
})
