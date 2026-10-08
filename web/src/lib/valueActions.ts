import type { RouteLocationRaw } from 'vue-router'
import { addFilter } from './searchQuery'

export interface ValueAction {
  label: string
  /** Where the action navigates. */
  to?: RouteLocationRaw
  /** Text copied to the clipboard. */
  copy?: string
}

export interface Here {
  path: string
  query: Record<string, unknown>
}

/** The search key for an attribute: service.name is searched as "service". */
function searchKey(key: string): string {
  return key === 'service.name' ? 'service' : key
}

function str(v: unknown): string {
  return typeof v === 'string' ? v : ''
}

/**
 * The actions offered by the value menu, the same everywhere in the UI:
 * every value leads to the logs, traces or page it relates to.
 */
export function valueActions(item: { key: string; value: string }, here: Here): ValueAction[] {
  const { key, value } = item
  const time: Record<string, string> = {}
  if (str(here.query.from)) time.from = str(here.query.from)
  if (str(here.query.to)) time.to = str(here.query.to)
  const actions: ValueAction[] = []

  if (key === 'trace_id') {
    actions.push({ label: 'Open trace', to: { path: `/traces/${value}` } })
  }
  if (key === 'service.name') {
    actions.push({ label: `Open service ${value}`, to: { path: `/services/${value}`, query: time } })
  }

  const k = searchKey(key)
  const v = key === 'level' ? value.toLowerCase() : value
  const inExplorer = here.path === '/logs' || here.path === '/traces'
  const forTraces = key !== 'level'

  if (inExplorer && (here.path === '/logs' || forTraces)) {
    const q = str(here.query.q)
    actions.push(
      { label: 'Filter on this value', to: { path: here.path, query: { ...here.query, q: addFilter(q, k, v) } } },
      { label: 'Exclude this value', to: { path: here.path, query: { ...here.query, q: addFilter(q, `-${k}`, v) } } },
    )
  }
  if (here.path !== '/logs') {
    actions.push({ label: 'View logs with this value', to: { path: '/logs', query: { ...time, q: addFilter('', k, v) } } })
  }
  if (forTraces && here.path !== '/traces') {
    actions.push({ label: 'View traces with this value', to: { path: '/traces', query: { ...time, q: addFilter('', k, v) } } })
  }
  actions.push({ label: 'Copy value', copy: value })
  return actions
}
