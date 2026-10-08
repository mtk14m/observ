import { reactive } from 'vue'
import { api, ApiError, type User } from './api'

/** The signed-in user, shared by the whole UI. */
export const session = reactive<{ user: User | null }>({ user: null })

interface GuardTarget {
  path: string
  fullPath: string
  meta: { public?: boolean } & Record<string, unknown>
}

/**
 * Router guard: public pages (sign in, setup) are always allowed; other
 * pages need a session, otherwise the visitor goes to sign in — or to the
 * setup page on a fresh install.
 */
export async function authGuard(to: GuardTarget): Promise<true | { path: string; query?: Record<string, string> }> {
  if (to.meta.public || session.user) return true
  try {
    session.user = await api.auth.me()
    return true
  } catch (e) {
    if (!(e instanceof ApiError) || e.status !== 401) throw e
  }
  const { setup_required } = await api.auth.status()
  if (setup_required) return { path: '/setup' }
  return { path: '/login', query: { next: to.fullPath } }
}

export async function signOut() {
  await api.auth.logout().catch(() => undefined)
  session.user = null
}
