import type { Span } from './api'

export interface WaterfallRow {
  span: Span
  depth: number
  /** Start of the span relative to the trace, between 0 and 1. */
  offset: number
  /** Duration relative to the trace, between 0 and 1. */
  width: number
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
  const visit = (s: Span, depth: number) => {
    rows.push({
      span: s,
      depth,
      offset: (s.start - start) / total,
      width: Math.max((s.end - s.start) / total, MIN_WIDTH),
    })
    for (const c of (children.get(s.span_id) ?? []).sort(byStart)) visit(c, depth + 1)
  }
  for (const r of roots.sort(byStart)) visit(r, 0)
  return rows
}
