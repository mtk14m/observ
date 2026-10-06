import { expect, test, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { api, type LogRecord } from '@/lib/api'
import { makeRouter } from '@/test/router'
import LogsView from './LogsView.vue'

vi.mock('@/lib/api', async (orig) => ({
  ...(await orig<typeof import('@/lib/api')>()),
  api: { logs: vi.fn(), logHistogram: vi.fn() },
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
