import { describe, expect, test } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import TimeRangePicker from './TimeRangePicker.vue'
import { makeRouter } from '@/test/router'

async function render(path = '/logs') {
  const router = await makeRouter(path)
  const wrapper = mount(TimeRangePicker, { global: { plugins: [router] }, attachTo: document.body })
  return { wrapper, router }
}

describe('TimeRangePicker', () => {
  test('shows the current range', async () => {
    const { wrapper } = await render('/logs?from=now-7d&to=now')
    expect(wrapper.get('button').text()).toContain('Past 7 days')
    wrapper.unmount()
  })

  test('selecting a preset updates the URL and closes the menu', async () => {
    const { wrapper, router } = await render()
    await wrapper.get('button').trigger('click')
    const options = wrapper.findAll('[role="option"]')
    expect(options.length).toBe(7)

    await options.find((o) => o.text() === 'Past 4 hours')!.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.query).toMatchObject({ from: 'now-4h', to: 'now' })
    expect(wrapper.find('[role="listbox"]').exists()).toBe(false)
    wrapper.unmount()
  })

  test('Escape closes the menu', async () => {
    const { wrapper } = await render()
    await wrapper.get('button').trigger('click')
    await wrapper.get('[role="listbox"]').trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('[role="listbox"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
