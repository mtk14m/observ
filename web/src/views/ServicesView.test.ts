import { expect, test, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { api } from '@/lib/api'
import { makeRouter } from '@/test/router'
import ServicesView from './ServicesView.vue'

vi.mock('@/lib/api', async (orig) => ({
  ...(await orig<typeof import('@/lib/api')>()),
  api: { services: vi.fn(), serviceMap: vi.fn().mockResolvedValue([]) },
}))


test('lists services with their RED metrics and links to their page', async () => {
  vi.mocked(api.services).mockResolvedValue([
    { name: 'payment', requests: 120, errors: 10, error_rate: 0.0833, rate_per_second: 2, p50_ms: 80, p95_ms: 190.5, p99_ms: 900 },
  ])
  const router = await makeRouter('/services?from=now-15m&to=now')
  const w = mount(ServicesView, { global: { plugins: [router] } })
  await flushPromises()

  expect(api.services).toHaveBeenCalledWith({ from: 'now-15m', to: 'now' })
  const cells = w.findAll('tbody tr td').map((td) => td.text())
  expect(cells).toEqual(['payment', '120', '2/s', '8.3%', '80 ms', '191 ms', '900 ms'])
  expect(w.find('tbody a').attributes('href')).toBe('/services/payment?from=now-15m&to=now')
  expect(w.find('.tab.active').text()).toContain('1')
})

test('explains how to send data when there is none', async () => {
  vi.mocked(api.services).mockResolvedValue([])
  const router = await makeRouter('/services')
  const w = mount(ServicesView, { global: { plugins: [router] } })
  await flushPromises()
  expect(w.text()).toContain('No services yet')
  expect(w.text()).toContain('OTEL_EXPORTER_OTLP_ENDPOINT')
})

test('shows API errors', async () => {
  vi.mocked(api.services).mockImplementation(() => Promise.reject(new Error('disk on fire')))
  const router = await makeRouter('/services')
  const w = mount(ServicesView, { global: { plugins: [router] } })
  await flushPromises()
  expect(w.find('[role="alert"]').text()).toContain('disk on fire')
})

test('draws the service map when services call each other', async () => {
  vi.mocked(api.services).mockResolvedValue([
    { name: 'web', requests: 1, errors: 0, error_rate: 0, rate_per_second: 1, p50_ms: 1, p95_ms: 1, p99_ms: 1 },
    { name: 'db', requests: 1, errors: 0, error_rate: 0, rate_per_second: 1, p50_ms: 1, p95_ms: 1, p99_ms: 1 },
  ])
  vi.mocked(api.serviceMap).mockResolvedValue([{ from: 'web', to: 'db', requests: 10, errors: 0 }])
  const router = await makeRouter('/services')
  const w = mount(ServicesView, { global: { plugins: [router] } })
  await flushPromises()
  expect(w.findAll('.service-map .node').map((n) => n.text())).toEqual(expect.arrayContaining([
    expect.stringContaining('web'), expect.stringContaining('db'),
  ]))
  expect(w.findAll('.service-map path.edge')).toHaveLength(1)
})
