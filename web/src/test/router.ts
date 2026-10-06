import { createMemoryHistory, createRouter, type Router } from 'vue-router'
import { routes } from '@/router'

/** Creates an isolated router for component tests, already at `path`. */
export async function makeRouter(path = '/'): Promise<Router> {
  const router = createRouter({ history: createMemoryHistory(), routes })
  await router.push(path)
  await router.isReady()
  return router
}
