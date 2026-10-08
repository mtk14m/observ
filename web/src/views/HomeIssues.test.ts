import { expect, test, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { api, type Issue, type RuleStatus, type ServiceSummary } from '@/lib/api'
import { makeRouter } from '@/test/router'
import HomeView from './HomeView.vue'
import IssuesView from './IssuesView.vue'

vi.mock('@/lib/api', async (orig) => ({
  ...(await orig<typeof import('@/lib/api')>()),
  api: { services: vi.fn(), alertRules: vi.fn(), issues: vi.fn(), deployments: vi.fn() },
}))

const svc = (name: string, o: Partial<ServiceSummary> = {}): ServiceSummary => ({
  name, requests: 100, errors: 0, error_rate: 0, rate_per_second: 1, p50_ms: 20, p95_ms: 100, p99_ms: 200, ...o,
})
const issue: Issue = {
  id: 'i1', service: 'payment', kind: 'log', title: 'payment refused', example: 'payment refused', count: 42,
  previous_count: 0, first_seen: 1e18, last_seen: 1e18, status: 'new', buckets: [0, 2, 5, 9], example_trace_id: 't1',
}
const firing = { id: 'r1', name: 'Payment refusals', status: 'firing', firing: 1 } as RuleStatus

function stub() {
  vi.mocked(api.services).mockImplementation(async (r) =>
    r.from === 'now-30m'
      ? [svc('inventory'), svc('checkout', { error_rate: 0.11, errors: 11 })]
      : [svc('inventory'), svc('checkout', { error_rate: 0.1 })])
  vi.mocked(api.alertRules).mockResolvedValue([firing, { ...firing, id: 'r2', name: 'Quiet', status: 'ok', firing: 0 }])
  vi.mocked(api.issues).mockResolvedValue([issue, { ...issue, id: 'i2', status: 'ongoing', title: 'old news' }])
  vi.mocked(api.deployments).mockResolvedValue([{ service: 'checkout', version: '1.4.2', previous: '1.4.1', at: 1e18 }])
}

test('Home says what needs attention and where to go next', async () => {
  stub()
  const router = await makeRouter('/?from=now-30m&to=now')
  const w = mount(HomeView, { global: { plugins: [router] } })
  await flushPromises()

  expect(w.find('h2').text()).toBe('1 service needs attention')
  const rows = w.findAll('.health tbody tr')
  expect(rows[0]!.text()).toContain('checkout')
  expect(rows[0]!.text()).toContain('11% errors')
  expect(rows[0]!.find('.light').classes()).toContain('critical')
  expect(rows[0]!.find('a').attributes('href')).toBe('/services/checkout?from=now-30m&to=now')

  const attention = w.find('.attention').text()
  expect(attention).toContain('Payment refusals')
  expect(attention).not.toContain('Quiet')
  expect(attention).toContain('payment refused')
  expect(attention).not.toContain('old news')
  expect(attention).toContain('checkout 1.4.1 → 1.4.2')
  expect(w.find('.attention a[href^="/issues"]').attributes('href')).toContain('issue=i1')
})

test('Home is calm when everything is fine', async () => {
  stub()
  vi.mocked(api.services).mockResolvedValue([svc('inventory')])
  vi.mocked(api.alertRules).mockResolvedValue([])
  vi.mocked(api.issues).mockResolvedValue([])
  vi.mocked(api.deployments).mockResolvedValue([])
  const router = await makeRouter('/')
  const w = mount(HomeView, { global: { plugins: [router] } })
  await flushPromises()
  expect(w.find('h2').text()).toBe('All services look healthy')
  expect(w.find('.attention').text()).toContain('Nothing needs attention')
})

test('Issues lists errors grouped, and a selected issue links to its records', async () => {
  stub()
  const router = await makeRouter('/issues?from=now-30m&to=now')
  const w = mount(IssuesView, { global: { plugins: [router] }, attachTo: document.body })
  await flushPromises()
  const rows = w.findAll('tbody tr')
  expect(rows).toHaveLength(2)
  expect(rows[0]!.text()).toContain('NEW')
  expect(rows[0]!.text()).toContain('payment refused')
  expect(rows[0]!.text()).toContain('42')
  expect(rows[0]!.findAll('.spark rect')).toHaveLength(4)

  await rows[0]!.trigger('click')
  await flushPromises()
  expect(router.currentRoute.value.query.issue).toBe('i1')
  const panel = w.find('[role="dialog"]')
  expect(panel.find('a[href^="/logs"]').exists()).toBe(true)
  expect(panel.find('a[href="/traces/t1"]').exists()).toBe(true)
})
