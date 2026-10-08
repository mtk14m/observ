import { expect, test, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { api, type ServiceDetail } from '@/lib/api'
import { makeRouter } from '@/test/router'
import ServiceView from './ServiceView.vue'

vi.mock('@/lib/api', async (orig) => ({
  ...(await orig<typeof import('@/lib/api')>()),
  api: {
    service: vi.fn(), logs: vi.fn(), logHistogram: vi.fn(), logFacet: vi.fn(), traces: vi.fn(),
    services: vi.fn(), issues: vi.fn(), compare: vi.fn(),
  },
}))

const detail: ServiceDetail = {
  summary: { name: 'checkout', requests: 1, errors: 0, error_rate: 0, rate_per_second: 1, p50_ms: 1, p95_ms: 1, p99_ms: 1 },
  step: 60e9, timeline: [], operations: [], calls: [], called_by: [],
}

async function render(path: string) {
  vi.mocked(api.service).mockResolvedValue(detail)
  vi.mocked(api.logs).mockResolvedValue([])
  vi.mocked(api.logHistogram).mockResolvedValue([])
  vi.mocked(api.traces).mockResolvedValue([])
  vi.mocked(api.issues).mockResolvedValue([
    { id: 'a', service: 'checkout', kind: 'log', title: 'mine', example: '', count: 1, previous_count: 0,
      first_seen: 0, last_seen: 0, status: 'ongoing', buckets: [], example_trace_id: '' },
    { id: 'b', service: 'payment', kind: 'log', title: 'not mine', example: '', count: 1, previous_count: 0,
      first_seen: 0, last_seen: 0, status: 'ongoing', buckets: [], example_trace_id: '' },
  ])
  const router = await makeRouter(path)
  const w = mount(ServiceView, { global: { plugins: [router] } })
  await flushPromises()
  return { w, router }
}

test('the service page is a hub with pre-filtered tabs', async () => {
  const { w, router } = await render('/services/checkout?from=now-1h&to=now')
  expect(w.findAll('.hub-tabs .tab').map((t) => t.text())).toEqual(['Overview', 'Traces', 'Logs', 'Issues'])

  await w.findAll('.hub-tabs .tab')[2]!.trigger('click')
  await flushPromises()
  expect(router.currentRoute.value.query.view).toBe('logs')
  expect(api.logs).toHaveBeenLastCalledWith({ from: 'now-1h', to: 'now' }, 'service:checkout', 200)
})

test('the scope combines with the search, without showing as a chip', async () => {
  const { w } = await render('/services/checkout?view=traces&q=status:error')
  expect(api.traces).toHaveBeenLastCalledWith({ from: 'now-1h', to: 'now' },
    expect.objectContaining({ q: 'service:checkout status:error' }))
  expect(w.findAll('.active-filter').map((c) => c.text())).toEqual(['status is error'])
})

test('the Issues tab only lists the issues of the service', async () => {
  const { w } = await render('/services/checkout?view=issues')
  const rows = w.findAll('tbody tr')
  expect(rows).toHaveLength(1)
  expect(rows[0]!.text()).toContain('mine')
})
