import { expect, test, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { api } from '@/lib/api'
import { makeRouter } from '@/test/router'
import ComparePanel from './ComparePanel.vue'

vi.mock('@/lib/api', async (orig) => ({
  ...(await orig<typeof import('@/lib/api')>()),
  api: { compare: vi.fn() },
}))

test('explains what distinguishes the errors, with clickable values', async () => {
  vi.mocked(api.compare).mockResolvedValue({
    selection_total: 84, baseline_total: 790,
    items: [
      { key: 'payment.issuer', value: 'acme-bank', selection: 0.92, baseline: 0.31 },
      { key: 'http.route', value: '/pay', selection: 1, baseline: 0.6 },
    ],
  })
  const router = await makeRouter('/traces?from=now-30m&to=now&q=service:checkout&tab=compare')
  const w = mount(ComparePanel, {
    props: { signal: 'spans', range: { from: 'now-30m', to: 'now' }, q: 'service:checkout' },
    global: { plugins: [router] },
  })
  await flushPromises()

  expect(api.compare).toHaveBeenCalledWith({ from: 'now-30m', to: 'now' },
    { signal: 'spans', q: 'service:checkout', errors: true, selFrom: undefined, selTo: undefined })
  expect(w.find('.headline').text()).toContain('84 spans in error')
  expect(w.find('.headline').text()).toContain('790 other spans')
  const rows = w.findAll('.row')
  expect(rows).toHaveLength(2)
  expect(rows[0]!.find('button.val').text()).toBe('acme-bank')
  expect(rows[0]!.text()).toContain('92%')
  expect(rows[0]!.text()).toContain('31%')
  expect((rows[0]!.find('.bar.sel').element as HTMLElement).style.width).toBe('92%')
})

test('compares a selected period when one is given', async () => {
  vi.mocked(api.compare).mockResolvedValue({ selection_total: 0, baseline_total: 10, items: [] })
  const router = await makeRouter('/logs')
  const w = mount(ComparePanel, {
    props: { signal: 'logs', range: { from: 'now-1h', to: 'now' }, q: '', selection: { from: 'a', to: 'b' } },
    global: { plugins: [router] },
  })
  await flushPromises()
  expect(api.compare).toHaveBeenLastCalledWith({ from: 'now-1h', to: 'now' },
    { signal: 'logs', q: '', errors: false, selFrom: 'a', selTo: 'b' })
  expect(w.text()).toContain('Nothing in the selected period')
})
