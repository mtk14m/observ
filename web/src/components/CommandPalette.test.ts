import { afterEach, expect, test, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import CommandPalette from './CommandPalette.vue'
import { makeRouter } from '@/test/router'

vi.mock('@/lib/api', async (orig) => ({
  ...(await orig<typeof import('@/lib/api')>()),
  api: {
    services: vi.fn().mockResolvedValue([{ name: 'payment' }]),
    metrics: vi.fn().mockResolvedValue([{ name: 'shop.orders' }]),
  },
}))

const Page = defineComponent({
  setup: () => () => h('div', [h('input', { 'data-search': '', class: 'page-search' }), h(CommandPalette)]),
})

const key = (k: string, opts: KeyboardEventInit = {}) =>
  document.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, ...opts }))

afterEach(() => {
  document.body.innerHTML = ''
})

async function render(path = '/logs?from=now-1h&to=now') {
  const router = await makeRouter(path)
  const w = mount(Page, { global: { plugins: [router] }, attachTo: document.body })
  return { w, router }
}

test('Ctrl+K opens the palette, Enter goes to the selected item', async () => {
  const { w, router } = await render()
  key('k', { ctrlKey: true })
  await flushPromises()
  const input = w.find('[role="dialog"] input')
  expect(input.exists()).toBe(true)
  await input.setValue('pay')
  await flushPromises()
  expect(w.findAll('[role="option"]')[0]!.text()).toBe('Service payment')
  await input.trigger('keydown', { key: 'Enter' })
  await flushPromises()
  expect(router.currentRoute.value.path).toBe('/services/payment')
  expect(w.find('[role="dialog"]').exists()).toBe(false)
})

test('arrow keys move the selection and Escape closes', async () => {
  const { w } = await render()
  key('k', { metaKey: true })
  await flushPromises()
  const input = w.find('[role="dialog"] input')
  await input.trigger('keydown', { key: 'ArrowDown' })
  expect(w.findAll('[role="option"]')[1]!.attributes('aria-selected')).toBe('true')
  await input.trigger('keydown', { key: 'Escape' })
  expect(w.find('[role="dialog"]').exists()).toBe(false)
})

test('/ focuses the page search and [ moves the period back', async () => {
  const { w, router } = await render()
  key('/')
  expect(document.activeElement).toBe(w.find('.page-search').element)

  ;(document.activeElement as HTMLElement).blur()
  key('[')
  await flushPromises()
  const q = router.currentRoute.value.query
  expect(typeof q.from === 'string' && q.from.endsWith('Z')).toBe(true)
})

test('shortcuts are ignored while typing', async () => {
  const { w, router } = await render()
  const input = w.find('.page-search').element as HTMLInputElement
  input.focus()
  input.dispatchEvent(new KeyboardEvent('keydown', { key: '[', bubbles: true }))
  await flushPromises()
  expect(router.currentRoute.value.query.from).toBe('now-1h')
})
