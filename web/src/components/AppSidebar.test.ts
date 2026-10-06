import { describe, expect, test } from 'vitest'
import { mount } from '@vue/test-utils'
import AppSidebar from './AppSidebar.vue'
import { makeRouter } from '@/test/router'

async function render(path: string) {
  const router = await makeRouter(path)
  return mount(AppSidebar, { global: { plugins: [router] } })
}

describe('AppSidebar', () => {
  test('lists every section in order', async () => {
    const wrapper = await render('/')
    const labels = wrapper.findAll('nav a').map((a) => a.attributes('aria-label'))
    expect(labels).toEqual(['Services', 'Traces', 'Logs', 'Metrics', 'Dashboards', 'Alerts'])
  })

  test('marks the current section', async () => {
    const wrapper = await render('/logs')
    const current = wrapper.find('nav a[aria-current="page"]')
    expect(current.attributes('aria-label')).toBe('Logs')
  })

  test('links keep the global time range', async () => {
    const wrapper = await render('/logs?from=now-7d&to=now&q=error')
    const traces = wrapper.find('nav a[aria-label="Traces"]')
    expect(traces.attributes('href')).toBe('/traces?from=now-7d&to=now')
  })

  test('collapses to icons only', async () => {
    const wrapper = await render('/')
    const toggle = wrapper.find('button[aria-controls="sidebar-nav"]')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(wrapper.classes()).toContain('collapsed')
  })
})
