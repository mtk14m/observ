import { expect, test, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { api, type Span } from '@/lib/api'
import { makeRouter } from '@/test/router'
import TraceView from './TraceView.vue'

vi.mock('@/lib/api', async (orig) => ({
  ...(await orig<typeof import('@/lib/api')>()),
  api: { trace: vi.fn(), logs: vi.fn() },
}))

const ms = 1e6
const t0 = 1_700_000_000_000 * ms
function span(id: string, parent: string, service: string, start: number, dur: number, error = false): Span {
  return {
    trace_id: 't1', span_id: id, parent_span_id: parent, name: `${service} op`, kind: 'Server',
    start: t0 + start * ms, end: t0 + (start + dur) * ms, duration_nano: dur * ms,
    status_code: error ? 'Error' : 'Unset', status_message: error ? 'card declined' : '',
    service, scope_name: '', attributes: { 'http.route': '/pay' }, resource_attributes: {},
    events: error ? [{ time_unix_nano: t0, name: 'exception', attributes: { 'exception.type': 'CardDeclined' } }] : [],
  }
}

async function render() {
  vi.mocked(api.trace).mockResolvedValue([span('a', '', 'frontend', 0, 200), span('b', 'a', 'payment', 20, 150, true)])
  vi.mocked(api.logs).mockResolvedValue([
    { time: t0, service: 'payment', severity: 'ERROR', severity_number: 17, body: 'payment refused',
      trace_id: 't1', span_id: 'b', attributes: {}, resource_attributes: {} },
  ])
  const router = await makeRouter('/traces/t1')
  const w = mount(TraceView, { global: { plugins: [router] }, attachTo: document.body })
  await flushPromises()
  return w
}

test('shows a summary, the waterfall and the logs of the trace', async () => {
  const w = await render()
  expect(api.trace).toHaveBeenCalledWith('t1')
  const summary = w.find('.summary').text()
  expect(summary).toContain('Max duration200 ms')
  expect(summary).toContain('Total spans2')
  expect(summary).toContain('Services2')
  expect(summary).toContain('Errors1')

  const rows = w.findAll('.span-row')
  expect(rows).toHaveLength(2)
  expect(rows[1]!.classes()).toContain('error')
  expect((rows[1]!.find('.name').element as HTMLElement).style.paddingLeft).toBe('20px')

  const [query] = vi.mocked(api.logs).mock.calls[0]!.slice(1)
  expect(query).toBe('trace_id:t1')
  expect(w.find('.logs').text()).toContain('payment refused')
})

test('clicking a span shows its attributes, status and events', async () => {
  const w = await render()
  await w.findAll('.span-row')[1]!.trigger('click')
  const panel = w.find('[role="dialog"]')
  expect(panel.text()).toContain('card declined')
  expect(panel.text()).toContain('CardDeclined')
  expect(panel.text()).toContain('http.route')
})

test('spans can be collapsed and searched', async () => {
  const w = await render()
  await w.findAll('.span-row')[0]!.find('button.toggle').trigger('click')
  expect(w.findAll('.span-row')).toHaveLength(1)
  await w.findAll('.span-row')[0]!.find('button.toggle').trigger('click')
  expect(w.findAll('.span-row')).toHaveLength(2)

  await w.find('input[aria-label="Search spans"]').setValue('frontend')
  expect(w.findAll('.span-row')).toHaveLength(1)
})

test('links to the logs of the trace', async () => {
  const w = await render()
  const href = w.find('a.view-logs').attributes('href')!
  expect(href).toContain('/logs?')
  expect(decodeURIComponent(href)).toContain('q=trace_id:t1')
})
