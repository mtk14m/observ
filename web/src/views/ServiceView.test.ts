import { expect, test, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { api, type ServiceDetail } from '@/lib/api'
import { makeRouter } from '@/test/router'
import ServiceView from './ServiceView.vue'

vi.mock('@/lib/api', async (orig) => ({
  ...(await orig<typeof import('@/lib/api')>()),
  api: { service: vi.fn() },
}))

const detail: ServiceDetail = {
  summary: { name: 'checkout', requests: 600, errors: 54, error_rate: 0.09, rate_per_second: 0.17, p50_ms: 110, p95_ms: 860, p99_ms: 940 },
  step: 60e9,
  timeline: [
    { t: 1e18, requests: 60, errors: 6, p50_ms: 100, p95_ms: 800, p99_ms: 900 },
    { t: 1e18 + 60e9, requests: 30, errors: 0, p50_ms: 120, p95_ms: 850, p99_ms: 950 },
  ],
  operations: [{ name: 'POST /checkout', requests: 600, errors: 54, p50_ms: 110, p95_ms: 860 }],
  calls: [{ service: 'payment', requests: 600, errors: 20 }],
  called_by: [{ service: 'frontend', requests: 600, errors: 0 }],
}

async function render() {
  vi.mocked(api.service).mockResolvedValue(detail)
  const router = await makeRouter('/services/checkout?from=now-1h&to=now')
  const w = mount(ServiceView, { global: { plugins: [router] } })
  await flushPromises()
  return w
}

test('shows the key numbers, charts, operations and dependencies', async () => {
  const w = await render()
  expect(api.service).toHaveBeenCalledWith({ from: 'now-1h', to: 'now' }, 'checkout')
  expect(w.findAll('.tile').map((t) => t.text())).toEqual([
    expect.stringContaining('0.17/s'), expect.stringContaining('9%'), expect.stringContaining('860 ms'),
  ])
  expect(w.findAll('.chart')).toHaveLength(3)
  // Requests per bucket of 60s become a rate: 60 requests → 1/s.
  expect(w.findAll('.chart')[0]!.text()).toContain('1/s')
  expect(w.find('.operations tbody').text()).toContain('POST /checkout')
  expect(w.find('a[href^="/services/payment"]').exists()).toBe(true)
  expect(w.find('a[href^="/services/frontend"]').exists()).toBe(true)
})

test('links to the traces and logs of the service', async () => {
  const w = await render()
  expect(w.find('a[href="/traces?from=now-1h&to=now&service=checkout"]').exists()).toBe(true)
  expect(w.find('a[href="/logs?from=now-1h&to=now&q=service:checkout"]').exists()).toBe(true)
})
