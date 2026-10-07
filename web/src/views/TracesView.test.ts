import { expect, test, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { api } from '@/lib/api'
import { makeRouter } from '@/test/router'
import TracesView from './TracesView.vue'

vi.mock('@/lib/api', async (orig) => ({
  ...(await orig<typeof import('@/lib/api')>()),
  api: { traces: vi.fn(), services: vi.fn() },
}))

async function render(path: string) {
  vi.mocked(api.services).mockResolvedValue([
    { name: 'frontend', requests: 1, errors: 0, error_rate: 0, rate_per_second: 1, p50_ms: 1, p95_ms: 1, p99_ms: 1 },
  ])
  vi.mocked(api.traces).mockResolvedValue([
    { trace_id: 't1', root_service: 'frontend', root_name: 'POST /checkout', start: 1e18, duration_nano: 250e6,
      span_count: 7, error_count: 1, services: ['frontend', 'payment'] },
  ])
  const router = await makeRouter(path)
  const w = mount(TracesView, { global: { plugins: [router] } })
  await flushPromises()
  return { w, router }
}

test('passes filters from the URL and links each trace', async () => {
  const { w } = await render('/traces?service=frontend&errors=true&min_ms=100')
  expect(api.traces).toHaveBeenLastCalledWith({ from: 'now-1h', to: 'now' },
    { service: 'frontend', errors: true, minDurationMs: 100 })
  const row = w.find('tbody tr')
  expect(row.text()).toContain('POST /checkout')
  expect(row.text()).toContain('250 ms')
  expect(row.find('a').attributes('href')).toBe('/traces/t1')
})

test('changing a filter updates the URL', async () => {
  const { w, router } = await render('/traces')
  const errorsOnly = w.find('button.errors-only')
  expect(errorsOnly.attributes('aria-pressed')).toBe('false')
  await errorsOnly.trigger('click')
  await flushPromises()
  expect(router.currentRoute.value.query.errors).toBe('true')

  await w.find('button.facet[data-key="service"]').trigger('click')
  await flushPromises()
  await w.find('.facet-menu [role="option"]').trigger('click')
  await flushPromises()
  expect(router.currentRoute.value.query.service).toBe('frontend')
  expect(w.find('.active-filter').text()).toContain('frontend')

  await w.find('input[aria-label="Minimum duration in ms"]').setValue('250')
  await w.find('input[aria-label="Minimum duration in ms"]').trigger('change')
  await flushPromises()
  expect(router.currentRoute.value.query.min_ms).toBe('250')
})

test('shows the number of traces', async () => {
  const { w } = await render('/traces')
  expect(w.find('.tab.active').text()).toContain('1')
})
