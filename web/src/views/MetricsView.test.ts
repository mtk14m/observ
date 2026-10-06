import { expect, test, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { api } from '@/lib/api'
import { makeRouter } from '@/test/router'
import MetricsView from './MetricsView.vue'

vi.mock('@/lib/api', async (orig) => ({
  ...(await orig<typeof import('@/lib/api')>()),
  api: { metrics: vi.fn(), queryMetric: vi.fn() },
}))

async function render(path: string) {
  vi.mocked(api.metrics).mockResolvedValue([
    { name: 'http.server.request.duration', type: 'Histogram', unit: 's', series: 6, attribute_keys: ['http.route', 'service.name'], monotonic: false },
    { name: 'shop.inventory.stock', type: 'Gauge', unit: '{item}', series: 5, attribute_keys: ['product', 'service.name'], monotonic: false },
  ])
  vi.mocked(api.queryMetric).mockResolvedValue([
    { labels: { 'service.name': 'payment' }, points: [{ t: 1e18, v: 0.2 }, { t: 1e18 + 6e10, v: 0.3 }] },
  ])
  const router = await makeRouter(path)
  const w = mount(MetricsView, { global: { plugins: [router] } })
  await flushPromises()
  return { w, router }
}

test('lists metrics and selects the first one by default', async () => {
  const { w } = await render('/metrics')
  expect(w.findAll('.metric-list button').map((b) => b.find('.metric-name').text())).toEqual([
    'http.server.request.duration', 'shop.inventory.stock',
  ])
  expect(api.queryMetric).toHaveBeenLastCalledWith({ from: 'now-1h', to: 'now' },
    { metric: 'http.server.request.duration', agg: 'p95', groupBy: [] })
})

test('the builder state lives in the URL', async () => {
  const { w, router } = await render('/metrics?metric=http.server.request.duration&agg=p99&by=service.name')
  expect(api.queryMetric).toHaveBeenLastCalledWith({ from: 'now-1h', to: 'now' },
    { metric: 'http.server.request.duration', agg: 'p99', groupBy: ['service.name'] })
  expect(w.find('path.line, .marks circle').exists()).toBe(true)

  await w.findAll('.metric-list button')[1]!.trigger('click')
  await flushPromises()
  expect(router.currentRoute.value.query).toMatchObject({ metric: 'shop.inventory.stock' })
  expect(router.currentRoute.value.query.agg).toBeUndefined()
})

test('filters the metric list', async () => {
  const { w } = await render('/metrics')
  await w.find('input[aria-label="Filter metrics"]').setValue('stock')
  expect(w.findAll('.metric-list button')).toHaveLength(1)
})
