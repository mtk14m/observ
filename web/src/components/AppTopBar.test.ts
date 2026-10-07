import { expect, test } from 'vitest'
import { mount } from '@vue/test-utils'
import AppTopBar from './AppTopBar.vue'
import { makeRouter } from '@/test/router'

test('shows the time range picker only on pages that use it', async () => {
  const logs = mount(AppTopBar, { global: { plugins: [await makeRouter('/logs')] } })
  expect(logs.find('button[aria-haspopup="listbox"]').exists()).toBe(true)

  for (const path of ['/alerts', '/traces/abc']) {
    const w = mount(AppTopBar, { global: { plugins: [await makeRouter(path)] } })
    expect(w.find('button[aria-haspopup="listbox"]').exists(), path).toBe(false)
  }
})
