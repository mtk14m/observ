import { describe, expect, test } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import TimeSeriesChart from './TimeSeriesChart.vue'

const s = 1e9
const series = [
  { label: 'checkout', points: [{ t: 10 * s, v: 5 }, { t: 20 * s, v: 8 }] },
  { label: 'payment', points: [{ t: 10 * s, v: 2 }, { t: 20 * s, v: 3 }] },
]

describe('TimeSeriesChart', () => {
  test('draws one line per series and a legend', () => {
    const w = mount(TimeSeriesChart, { props: { series, from: 0, to: 30 * s } })
    expect(w.findAll('path.line')).toHaveLength(2)
    expect(w.findAll('.legend li').map((li) => li.find('.label').text())).toEqual(['checkout', 'payment'])
  })

  test('a single series has no legend, a single point is a dot', () => {
    const w = mount(TimeSeriesChart, {
      props: { series: [{ label: 'only', points: [{ t: 10 * s, v: 1 }] }], from: 0, to: 30 * s },
    })
    expect(w.find('.legend').exists()).toBe(false)
    expect(w.findAll('.marks circle')).toHaveLength(1)
  })

  test('hover shows every series value at the nearest time, largest first', async () => {
    const w = mount(TimeSeriesChart, {
      props: { series, from: 0, to: 30 * s, format: (v: number) => `${v} req/s` },
    })
    // Plot width is 640 - 56 - 12 = 572px for 30s: t=20s is at ~381px from the plot's left edge.
    w.find('rect.overlay').element.dispatchEvent(new MouseEvent('pointermove', { clientX: 380 }))
    await nextTick()
    const rows = w.findAll('.tooltip-row').map((r) => r.text())
    expect(rows).toEqual(['8 req/scheckout', '3 req/spayment'])

    await w.find('rect.overlay').trigger('pointerleave')
    expect(w.find('.tooltip').exists()).toBe(false)
  })
})

test('breaks the line across gaps in the data instead of inventing values', () => {
  const gappy = [{ label: 'x', points: [0, 10, 20, 80, 90].map((t) => ({ t: t * s, v: 1 })) }]
  const w = mount(TimeSeriesChart, { props: { series: gappy, from: 0, to: 100 * s } })
  const d = w.find('path.line').attributes('d')!
  expect(d.match(/M/g)).toHaveLength(2)
})

describe('drag to zoom', () => {
  // Plot spans 56px → 628px (572px) for [0s, 30s): x = 56 + t/30 × 572.
  const at = (sec: number) => 56 + (sec / 30) * 572

  test('dragging across the plot emits the selected period', async () => {
    const w = mount(TimeSeriesChart, { props: { series, from: 0, to: 30 * s } })
    const overlay = w.find('rect.overlay').element
    // clientX is relative to the overlay, which starts at the plot's left edge.
    overlay.dispatchEvent(new MouseEvent('pointerdown', { clientX: at(10) - 56, bubbles: true }))
    overlay.dispatchEvent(new MouseEvent('pointermove', { clientX: at(20) - 56, bubbles: true }))
    await nextTick()
    expect(w.find('rect.selection').exists()).toBe(true)
    overlay.dispatchEvent(new MouseEvent('pointerup', { clientX: at(20) - 56, bubbles: true }))
    const [[range]] = w.emitted('zoom') as [[{ from: number; to: number }]]
    expect(Math.round(range.from / s)).toBe(10)
    expect(Math.round(range.to / s)).toBe(20)
  })

  test('a click without dragging does not zoom', () => {
    const w = mount(TimeSeriesChart, { props: { series, from: 0, to: 30 * s } })
    const overlay = w.find('rect.overlay').element
    overlay.dispatchEvent(new MouseEvent('pointerdown', { clientX: 100, bubbles: true }))
    overlay.dispatchEvent(new MouseEvent('pointerup', { clientX: 101, bubbles: true }))
    expect(w.emitted('zoom')).toBeUndefined()
  })
})
