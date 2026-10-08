import { expect, test, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { api, type LogRecord } from '@/lib/api'
import { makeRouter } from '@/test/router'
import LogsView from './LogsView.vue'

vi.mock('@/lib/api', async (orig) => ({
  ...(await orig<typeof import('@/lib/api')>()),
  api: { logs: vi.fn(), logHistogram: vi.fn(), logFacet: vi.fn(), compare: vi.fn() },
}))

const record: LogRecord = {
  time: 1_700_000_000_000_000_000, service: 'payment', severity: 'ERROR', severity_number: 17,
  body: 'payment refused', trace_id: 'abc123', span_id: 's1',
  attributes: { 'order.id': '42' }, resource_attributes: { 'service.name': 'payment' },
}

async function render(path: string) {
  vi.mocked(api.logs).mockResolvedValue([record])
  vi.mocked(api.logHistogram).mockResolvedValue([])
  const router = await makeRouter(path)
  const w = mount(LogsView, { global: { plugins: [router] }, attachTo: document.body })
  await flushPromises()
  return { w, router }
}

test('searches with the query from the URL', async () => {
  const { w } = await render('/logs?q=level:error')
  expect(api.logs).toHaveBeenLastCalledWith({ from: 'now-1h', to: 'now' }, 'level:error', 200)
  expect((w.find('input[type="search"]').element as HTMLInputElement).value).toBe('level:error')
  expect(w.find('tbody tr').text()).toContain('payment refused')
})

test('submitting the search writes it to the URL', async () => {
  const { w, router } = await render('/logs')
  await w.find('input[type="search"]').setValue('service:payment')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(router.currentRoute.value.query.q).toBe('service:payment')
})

test('clicking a record opens its details with a link to the trace', async () => {
  const { w } = await render('/logs')
  await w.find('tbody tr').trigger('click')
  const panel = w.find('[role="dialog"]')
  expect(panel.text()).toContain('order.id')
  expect(panel.find('a[href^="/traces/abc123"]').exists()).toBe(true)
})

test('shows search syntax errors', async () => {
  vi.mocked(api.logs).mockImplementation(() => Promise.reject(new Error('logsearch: syntax error: unterminated quote')))
  vi.mocked(api.logHistogram).mockResolvedValue([])
  const router = await makeRouter('/logs?q=%22bad')
  const w = mount(LogsView, { global: { plugins: [router] } })
  await flushPromises()
  expect(w.find('[role="alert"]').text()).toContain('unterminated quote')
})

test('active filters are chips that can be removed', async () => {
  const { w, router } = await render('/logs?q=service:payment%20level:error%20timeout')
  const chips = w.findAll('.active-filter')
  expect(chips.map((c) => c.text())).toEqual(['service is payment', 'level is error'])
  await chips[1]!.find('button').trigger('click')
  await flushPromises()
  expect(router.currentRoute.value.query.q).toBe('service:payment timeout')
})

test('facets list values with counts and add a filter', async () => {
  vi.mocked(api.logFacet).mockResolvedValue([{ value: 'checkout', count: 12 }, { value: 'payment', count: 5 }])
  const { w, router } = await render('/logs?q=timeout')
  await w.find('button.facet[data-key="service.name"]').trigger('click')
  await flushPromises()
  expect(api.logFacet).toHaveBeenLastCalledWith({ from: 'now-1h', to: 'now' }, 'timeout', 'service.name')
  const options = w.findAll('.facet-menu [role="option"]')
  expect(options.map((o) => o.text())).toEqual(['checkout12', 'payment5'])
  await options[0]!.trigger('click')
  await flushPromises()
  expect(router.currentRoute.value.query.q).toBe('timeout service:checkout')
})

test('shows the number of matching logs and level pills', async () => {
  vi.mocked(api.logHistogram).mockResolvedValue([{ t: 0, counts: { ERROR: 3, INFO: 4 } }])
  const router = await makeRouter('/logs')
  vi.mocked(api.logs).mockResolvedValue([record])
  const w = mount(LogsView, { global: { plugins: [router] } })
  await flushPromises()
  expect(w.find('.tab.active').text()).toContain('7')
  expect(w.find('tbody .pill.error').text()).toBe('Error')
})

test('in the Compare tab, dragging on the chart selects a period to compare', async () => {
  vi.mocked(api.compare).mockResolvedValue({ selection_total: 1, baseline_total: 1, items: [] })
  vi.mocked(api.logHistogram).mockResolvedValue([{ t: 0, counts: { INFO: 1 } }])
  const { w, router } = await render('/logs?tab=compare')
  expect(w.find('.compare').exists()).toBe(true)
  w.findComponent({ name: 'StackedBars' }).vm.$emit('zoom', { from: 1e18, to: 1e18 + 6e10 })
  await flushPromises()
  const q = router.currentRoute.value.query
  expect(q.sel_from).toBe(new Date(1e12).toISOString())
  expect(q.from).toBeUndefined() // the global range did not change
})
