import { expect, test, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { api, type RuleStatus } from '@/lib/api'
import { makeRouter } from '@/test/router'
import AlertsView from './AlertsView.vue'

vi.mock('@/lib/api', async (orig) => ({
  ...(await orig<typeof import('@/lib/api')>()),
  api: {
    alertRules: vi.fn(), alertEvents: vi.fn(), channels: vi.fn(), metrics: vi.fn(),
    createRule: vi.fn(), updateRule: vi.fn(), deleteRule: vi.fn(), previewRule: vi.fn(),
    createChannel: vi.fn(), deleteChannel: vi.fn(), testChannel: vi.fn(),
  },
}))

const firing: RuleStatus = {
  id: 'r1', name: 'Checkout errors', kind: 'logs', query: 'service:checkout level:error', op: '>', threshold: 5,
  window_seconds: 300, for_seconds: 0, channels: ['c1'], enabled: true, status: 'firing', firing: 1,
  states: [{ rule_id: 'r1', group: '', labels: {}, status: 'firing', since: '2026-10-07T12:00:00Z', value: 12 }],
}

async function render(path = '/alerts') {
  vi.mocked(api.alertRules).mockResolvedValue([firing])
  vi.mocked(api.alertEvents).mockResolvedValue([
    { id: 1, rule_id: 'r1', rule_name: 'Checkout errors', labels: {}, status: 'firing', value: 12, at: '2026-10-07T12:00:00Z' },
  ])
  vi.mocked(api.channels).mockResolvedValue([{ id: 'c1', name: 'ops', type: 'slack', url: 'https://hooks.slack.com/x' }])
  vi.mocked(api.metrics).mockResolvedValue([])
  const router = await makeRouter(path)
  const w = mount(AlertsView, { global: { plugins: [router] }, attachTo: document.body })
  await flushPromises()
  return { w, router }
}

test('lists rules with their status, condition and value', async () => {
  const { w } = await render()
  const row = w.find('tbody tr')
  expect(row.text()).toContain('FIRING')
  expect(row.text()).toContain('Checkout errors')
  expect(row.text()).toContain('count of logs matching service:checkout level:error > 5 over 5m')
  expect(row.text()).toContain('12')
  expect(w.find('.events').text()).toContain('Checkout errors')
})

test('creates a rule after previewing it', async () => {
  vi.mocked(api.previewRule).mockResolvedValue([{ labels: {}, value: 3, breached: false }])
  vi.mocked(api.createRule).mockResolvedValue({ ...firing, id: 'r2' })
  const { w } = await render()

  await w.find('button.new-rule').trigger('click')
  await flushPromises() // the form opens through the URL (?rule=new)
  const form = w.find('form.rule-form')
  await form.find('input[name="name"]').setValue('Payment refusals')
  await form.find('input[name="query"]').setValue('service:payment "refused"')
  await form.find('input[name="threshold"]').setValue('10')
  await form.find('select[name="window"]').setValue('900')
  await form.find('input[type="checkbox"][value="c1"]').setValue(true)

  await form.find('button.preview').trigger('click')
  await flushPromises()
  expect(form.find('.preview-result').text()).toContain('3')
  expect(form.find('.preview-result').text()).toContain('OK')

  await form.trigger('submit')
  await flushPromises()
  expect(api.createRule).toHaveBeenCalledWith(expect.objectContaining({
    name: 'Payment refusals', kind: 'logs', query: 'service:payment "refused"', op: '>', threshold: 10,
    window_seconds: 900, for_seconds: 0, channels: ['c1'], enabled: true,
  }))
  expect(w.find('form.rule-form').exists()).toBe(false)
})

test('shows validation errors from the server', async () => {
  vi.mocked(api.createRule).mockImplementation(() => Promise.reject(new Error('alert: invalid: name is required')))
  const { w } = await render()
  await w.find('button.new-rule').trigger('click')
  await flushPromises()
  await w.find('form.rule-form').trigger('submit')
  await flushPromises()
  expect(w.find('form.rule-form [role="alert"]').text()).toContain('name is required')
})

test('a link with ?rule= opens that rule', async () => {
  const { w } = await render('/alerts?rule=r1')
  expect((w.find('form.rule-form input[name="name"]').element as HTMLInputElement).value).toBe('Checkout errors')
})

test('channels can be added and tested', async () => {
  vi.mocked(api.createChannel).mockResolvedValue({ id: 'c2', name: 'pager', type: 'webhook', url: 'https://x.dev/h' })
  vi.mocked(api.testChannel).mockResolvedValue({ result: 'sent' })
  const { w } = await render()
  const panel = w.find('.channels')
  await panel.find('input[name="channel-name"]').setValue('pager')
  await panel.find('select[name="channel-type"]').setValue('webhook')
  await panel.find('input[name="channel-url"]').setValue('https://x.dev/h')
  await panel.find('form').trigger('submit')
  await flushPromises()
  expect(api.createChannel).toHaveBeenCalledWith({ name: 'pager', type: 'webhook', url: 'https://x.dev/h' })

  await panel.find('button.test').trigger('click')
  await flushPromises()
  expect(api.testChannel).toHaveBeenCalledWith('c1')
  expect(w.find('.channels').text()).toContain('Sent')
})
