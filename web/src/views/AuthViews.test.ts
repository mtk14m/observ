import { beforeEach, expect, test, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { api, ApiError } from '@/lib/api'
import { session } from '@/lib/session'
import { makeRouter } from '@/test/router'
import LoginView from './LoginView.vue'
import SetupView from './SetupView.vue'
import SettingsView from './SettingsView.vue'

vi.mock('@/lib/api', async (orig) => {
  const mod = await orig<typeof import('@/lib/api')>()
  return {
    ...mod,
    api: {
      auth: { login: vi.fn(), setup: vi.fn(), changePassword: vi.fn() },
      users: vi.fn(), createUser: vi.fn(), deleteUser: vi.fn(),
    },
  }
})

const ada = { id: 'u1', email: 'ada@example.com', name: 'Ada', role: 'admin' as const }

beforeEach(() => {
  vi.clearAllMocks()
  session.user = null
})

test('sign in goes back to the page that was asked for', async () => {
  vi.mocked(api.auth.login).mockResolvedValue(ada)
  const router = await makeRouter('/login?next=/logs%3Fq%3Derror')
  const w = mount(LoginView, { global: { plugins: [router] } })
  await w.find('input[type="email"]').setValue('ada@example.com')
  await w.find('input[type="password"]').setValue('correct horse battery')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(api.auth.login).toHaveBeenCalledWith('ada@example.com', 'correct horse battery')
  expect(session.user).toEqual(ada)
  expect(router.currentRoute.value.fullPath).toBe('/logs?q=error')
})

test('sign in shows errors and never redirects elsewhere', async () => {
  vi.mocked(api.auth.login).mockImplementation(() => Promise.reject(new ApiError('auth: invalid email or password', 401)))
  const router = await makeRouter('/login?next=https://evil.example')
  const w = mount(LoginView, { global: { plugins: [router] } })
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(w.find('[role="alert"]').text()).toContain('invalid email or password')

  vi.mocked(api.auth.login).mockResolvedValue(ada)
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(router.currentRoute.value.path).toBe('/') // an external "next" is ignored
})

test('setup creates the admin account', async () => {
  vi.mocked(api.auth.setup).mockResolvedValue(ada)
  const router = await makeRouter('/setup')
  const w = mount(SetupView, { global: { plugins: [router] } })
  await w.find('input[name="name"]').setValue('Ada')
  await w.find('input[name="email"]').setValue('ada@example.com')
  await w.find('input[name="password"]').setValue('correct horse battery')
  await w.find('input[name="confirm"]').setValue('correct horse batterx')
  await w.find('form').trigger('submit')
  expect(w.find('[role="alert"]').text()).toContain('do not match')
  expect(api.auth.setup).not.toHaveBeenCalled()

  await w.find('input[name="confirm"]').setValue('correct horse battery')
  await w.find('form').trigger('submit')
  await flushPromises()
  expect(api.auth.setup).toHaveBeenCalledWith({ name: 'Ada', email: 'ada@example.com', password: 'correct horse battery' })
  expect(session.user).toEqual(ada)
  expect(router.currentRoute.value.path).toBe('/')
})

test('settings: change password, and admins manage users', async () => {
  session.user = ada
  vi.mocked(api.users).mockResolvedValue([ada, { id: 'u2', email: 'bob@example.com', name: 'Bob', role: 'member' }])
  vi.mocked(api.auth.changePassword).mockResolvedValue(undefined)
  vi.mocked(api.createUser).mockResolvedValue({ id: 'u3', email: 'cy@example.com', name: 'Cy', role: 'member' })
  vi.mocked(api.deleteUser).mockResolvedValue(undefined)
  const router = await makeRouter('/settings')
  const w = mount(SettingsView, { global: { plugins: [router] } })
  await flushPromises()

  const pw = w.find('form.password')
  await pw.find('input[name="current"]').setValue('correct horse battery')
  await pw.find('input[name="new"]').setValue('an even better password')
  await pw.trigger('submit')
  await flushPromises()
  expect(api.auth.changePassword).toHaveBeenCalledWith('correct horse battery', 'an even better password')
  expect(pw.text()).toContain('Password changed')

  const rows = w.findAll('.users tbody tr')
  expect(rows).toHaveLength(2)
  expect(rows[0]!.find('button.delete').exists()).toBe(false) // not yourself
  await rows[1]!.find('button.delete').trigger('click')
  await flushPromises()
  expect(api.deleteUser).toHaveBeenCalledWith('u2')

  const add = w.find('form.add-user')
  await add.find('input[name="name"]').setValue('Cy')
  await add.find('input[name="email"]').setValue('cy@example.com')
  await add.find('input[name="password"]').setValue('cys long password')
  await add.find('select[name="role"]').setValue('member')
  await add.trigger('submit')
  await flushPromises()
  expect(api.createUser).toHaveBeenCalledWith({ name: 'Cy', email: 'cy@example.com', password: 'cys long password', role: 'member' })
})

test('members do not see user management', async () => {
  session.user = { ...ada, role: 'member' }
  const router = await makeRouter('/settings')
  const w = mount(SettingsView, { global: { plugins: [router] } })
  await flushPromises()
  expect(w.find('.users').exists()).toBe(false)
  expect(api.users).not.toHaveBeenCalled()
})
