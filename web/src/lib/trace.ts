import type { Span } from './api'

export interface WaterfallRow {
  span: Span
  depth: number
  /** Start of the span relative to the trace, between 0 and 1. */
  offset: number
  /** Duration relative to the trace, between 0 and 1. */
  width: number
  hasChildren: boolean
  /** Span IDs of the ancestors, root first. */
  ancestors: string[]
}

const MIN_WIDTH = 0.002

/** Orders spans depth-first (children by start time) for a waterfall view. */
export function buildWaterfall(spans: Span[]): WaterfallRow[] {
  if (spans.length === 0) return []
  const ids = new Set(spans.map((s) => s.span_id))
  const children = new Map<string, Span[]>()
  const roots: Span[] = []
  for (const s of spans) {
    if (s.parent_span_id && ids.has(s.parent_span_id)) {
      const list = children.get(s.parent_span_id) ?? []
      list.push(s)
      children.set(s.parent_span_id, list)
    } else {
      roots.push(s)
    }
  }
  const byStart = (a: Span, b: Span) => a.start - b.start
  const start = Math.min(...spans.map((s) => s.start))
  const end = Math.max(...spans.map((s) => s.end))
  const total = Math.max(end - start, 1)

  const rows: WaterfallRow[] = []
  const visit = (s: Span, ancestors: string[]) => {
    const kids = (children.get(s.span_id) ?? []).sort(byStart)
    rows.push({
      span: s,
      depth: ancestors.length,
      offset: (s.start - start) / total,
      width: Math.max((s.end - s.start) / total, MIN_WIDTH),
      hasChildren: kids.length > 0,
      ancestors,
    })
    for (const c of kids) visit(c, [...ancestors, s.span_id])
  }
  for (const r of roots.sort(byStart)) visit(r, [])
  return rows
}

/**
 * Filters waterfall rows: descendants of collapsed spans are hidden, and a
 * search (on service and operation names) keeps matches and their ancestors.
 */
export function visibleRows(rows: WaterfallRow[], collapsed: Set<string>, search: string): WaterfallRow[] {
  let out = rows.filter((r) => !r.ancestors.some((id) => collapsed.has(id)))
  const needle = search.trim().toLowerCase()
  if (needle) {
    const keep = new Set<string>()
    for (const r of out) {
      if (`${r.span.service} ${r.span.name}`.toLowerCase().includes(needle)) {
        keep.add(r.span.span_id)
        r.ancestors.forEach((a) => keep.add(a))
      }
    }
    out = out.filter((r) => keep.has(r.span.span_id))
  }
  return out
}
