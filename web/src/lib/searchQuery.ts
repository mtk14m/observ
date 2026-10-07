/**
 * Reads and edits log search queries (see the Go package logsearch for the
 * syntax), so that filters can be shown as chips and added from facets.
 */

export interface Filter {
  /** The term exactly as written in the query. */
  raw: string
  key: string
  value: string
  negated: boolean
}

// A term: optional "-", then a quoted phrase or a key:value pair or a word.
const TERM = /-?(?:"[^"]*"|[^\s:"]+:(?:"[^"]*"|\S+)|\S+)/g
const FILTER = /^(-?)([^\s:"]+):(?:"([^"]*)"|(\S+))$/

export function filters(query: string): Filter[] {
  const out: Filter[] = []
  const terms: string[] = query.match(TERM) ?? []
  for (const raw of terms) {
    const m = FILTER.exec(raw)
    if (m) out.push({ raw, key: m[2]!, value: m[3] ?? m[4]!, negated: m[1] === '-' })
  }
  return out
}

export function addFilter(query: string, key: string, value: string): string {
  const term = `${key}:${/\s/.test(value) ? `"${value}"` : value}`
  const terms: string[] = query.match(TERM) ?? []
  if (terms.includes(term)) return query
  return query.trim() ? `${query.trim()} ${term}` : term
}

export function removeFilter(query: string, raw: string): string {
  const terms: string[] = query.match(TERM) ?? []
  const i = terms.indexOf(raw)
  if (i >= 0) terms.splice(i, 1)
  return terms.join(' ')
}
