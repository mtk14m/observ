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

test('shows the section icon, title and a breadcrumb on detail pages', async () => {
  const logs = mount(AppTopBar, { global: { plugins: [await makeRouter('/logs')] } })
  expect(logs.find('h1').text()).toBe('Logs explorer')
  expect(logs.find('header svg').exists()).toBe(true)

  const trace = mount(AppTopBar, { global: { plugins: [await makeRouter('/traces/0af7651916cd43dd8448eb211c80319c')] } })
  expect(trace.find('.crumb').text()).toBe('Traces explorer')
  expect(trace.find('.crumb').attributes('href')).toBe('/traces')
  expect(trace.find('h1').text()).toBe('0af76519')

  const service = mount(AppTopBar, { global: { plugins: [await makeRouter('/services/checkout')] } })
  expect(service.find('h1').text()).toBe('checkout')
})
