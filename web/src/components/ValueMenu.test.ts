import { afterEach, expect, test, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import Val from './Val.vue'
import ValueMenu from './ValueMenu.vue'
import { makeRouter } from '@/test/router'

// A page with a value and the shell's single menu.
const Page = defineComponent({
  setup: () => () => h('div', [h(Val, { k: 'payment.issuer', v: 'acme-bank' }), h(ValueMenu)]),
})

afterEach(() => vi.unstubAllGlobals())

async function render(path: string) {
  const router = await makeRouter(path)
  const w = mount(Page, { global: { plugins: [router] }, attachTo: document.body })
  return { w, router }
}

test('clicking a value opens its menu, and an action navigates', async () => {
  const { w, router } = await render('/logs?from=now-30m&to=now&q=timeout')
  expect(w.find('[role="menu"]').exists()).toBe(false)
  await w.find('button.val').trigger('click')
  const items = w.findAll('[role="menu"] [role="menuitem"]')
  expect(items.map((i) => i.text())).toContain('Filter on this value')
  expect(w.find('[role="menu"]').text()).toContain('payment.issuer')

  await items.find((i) => i.text() === 'Filter on this value')!.trigger('click')
  await flushPromises()
  expect(router.currentRoute.value.query.q).toBe('timeout payment.issuer:acme-bank')
  expect(w.find('[role="menu"]').exists()).toBe(false)
})

test('copy writes to the clipboard and Escape closes the menu', async () => {
  const writeText = vi.fn().mockResolvedValue(undefined)
  vi.stubGlobal('navigator', { clipboard: { writeText } })
  const { w } = await render('/logs')
  await w.find('button.val').trigger('click')
  await w.findAll('[role="menuitem"]').find((i) => i.text() === 'Copy value')!.trigger('click')
  expect(writeText).toHaveBeenCalledWith('acme-bank')

  await w.find('button.val').trigger('click')
  document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
  await flushPromises()
  expect(w.find('[role="menu"]').exists()).toBe(false)
})
