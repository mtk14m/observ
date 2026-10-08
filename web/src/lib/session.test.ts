import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'
import { api, onUnauthorized } from './api'
import { authGuard, session } from './session'

function respond(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status })
}

beforeEach(() => {
  session.user = null
})
afterEach(() => {
  vi.unstubAllGlobals()
  onUnauthorized(null)
})

describe('api', () => {
  test('a 401 on a data endpoint triggers the unauthorized handler', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(respond(401, { error: 'sign in required' })))
    const handler = vi.fn()
    onUnauthorized(handler)
    await expect(api.services({ from: 'now-1h', to: 'now' })).rejects.toThrow('sign in required')
    expect(handler).toHaveBeenCalledOnce()
  })

  test('a 401 from sign-in itself does not', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(respond(401, { error: 'invalid email or password' })))
    const handler = vi.fn()
    onUnauthorized(handler)
    await expect(api.auth.login('a@b.c', 'x')).rejects.toThrow('invalid email or password')
    expect(handler).not.toHaveBeenCalled()
  })
})

describe('authGuard', () => {
  test('public pages are always allowed', async () => {
    expect(await authGuard({ path: '/login', fullPath: '/login', meta: { public: true } })).toBe(true)
  })

  test('loads the signed-in user once', async () => {
    const fetch = vi.fn().mockResolvedValue(respond(200, { data: { id: 'u', email: 'a@b.c', name: 'A', role: 'admin' } }))
    vi.stubGlobal('fetch', fetch)
    expect(await authGuard({ path: '/logs', fullPath: '/logs?q=x', meta: {} })).toBe(true)
    expect(session.user?.email).toBe('a@b.c')
    await authGuard({ path: '/traces', fullPath: '/traces', meta: {} })
    expect(fetch).toHaveBeenCalledOnce()
  })

  test('sends visitors to sign in, or to setup on a fresh install', async () => {
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(respond(401, { error: 'sign in required' }))
      .mockResolvedValueOnce(respond(200, { data: { setup_required: false } })))
    expect(await authGuard({ path: '/logs', fullPath: '/logs?q=x', meta: {} }))
      .toEqual({ path: '/login', query: { next: '/logs?q=x' } })

    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(respond(401, { error: 'sign in required' }))
      .mockResolvedValueOnce(respond(200, { data: { setup_required: true } })))
    expect(await authGuard({ path: '/logs', fullPath: '/logs', meta: {} })).toEqual({ path: '/setup' })
  })
})
