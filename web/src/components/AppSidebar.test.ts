import { describe, expect, test, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
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
    const current = wrapper.find('nav a.active')
    expect(current.attributes('aria-label')).toBe('Logs')
  })

  test('links keep the global time range', async () => {
    const wrapper = await render('/logs?from=now-7d&to=now&q=error')
    const traces = wrapper.find('nav a[aria-label="Traces"]')
    expect(traces.attributes('href')).toBe('/traces?from=now-7d&to=now')
  })

  test('groups sections and toggles the theme', async () => {
    const wrapper = await render('/')
    expect(wrapper.findAll('nav .group')).toHaveLength(3)
    const toggle = wrapper.find('button.theme')
    const before = toggle.attributes('aria-label')
    await toggle.trigger('click')
    expect(toggle.attributes('aria-label')).not.toBe(before)
  })
})

describe('user menu', () => {
  test('shows the user and signs out', async () => {
    const { session } = await import('@/lib/session')
    session.user = { id: 'u1', email: 'ada@example.com', name: 'Ada Lovelace', role: 'admin' }
    const fetch = vi.fn().mockResolvedValue(new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetch)

    const router = await makeRouter('/logs')
    const wrapper = mount(AppSidebar, { global: { plugins: [router] } })
    const avatar = wrapper.find('button.avatar')
    expect(avatar.text()).toBe('A')
    await avatar.trigger('click')
    const menu = wrapper.find('.user-menu')
    expect(menu.text()).toContain('Ada Lovelace')
    expect(menu.text()).toContain('ada@example.com')
    expect(menu.find('a[href="/settings"]').exists()).toBe(true)

    await menu.find('button.sign-out').trigger('click')
    await flushPromises()
    expect(fetch.mock.calls[0]![0]).toBe('/api/v1/auth/logout')
    expect(session.user).toBeNull()
    expect(router.currentRoute.value.path).toBe('/login')
    vi.unstubAllGlobals()
  })
})
