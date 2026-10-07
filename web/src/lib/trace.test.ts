import { describe, expect, test } from 'vitest'
import { buildWaterfall, visibleRows } from './trace'
import type { Span } from './api'

function span(id: string, parent: string, start: number, end: number, service = 'svc'): Span {
  return {
    trace_id: 't', span_id: id, parent_span_id: parent, name: id, kind: 'Server',
    start, end, duration_nano: end - start, status_code: 'Unset', status_message: '',
    service, scope_name: '', attributes: {}, resource_attributes: {}, events: [],
  }
}

describe('buildWaterfall', () => {
  test('orders spans depth-first and computes offsets', () => {
    const rows = buildWaterfall([
      span('child2', 'root', 60, 100),
      span('root', '', 0, 100),
      span('grandchild', 'child1', 20, 30),
      span('child1', 'root', 10, 50),
    ])
    expect(rows.map((r) => [r.span.span_id, r.depth])).toEqual([
      ['root', 0], ['child1', 1], ['grandchild', 2], ['child2', 1],
    ])
    expect(rows[1]).toMatchObject({ offset: 0.1, width: 0.4 })
  })

  test('treats spans with a missing parent as roots', () => {
    const rows = buildWaterfall([span('orphan', 'gone', 5, 10), span('root', '', 0, 10)])
    expect(rows.map((r) => [r.span.span_id, r.depth])).toEqual([['root', 0], ['orphan', 0]])
  })

  test('gives zero-length traces a visible width', () => {
    const rows = buildWaterfall([span('a', '', 5, 5)])
    expect(rows[0]!.width).toBeGreaterThan(0)
  })
})

describe('visibleRows', () => {
  const rows = buildWaterfall([
    span('root', '', 0, 100),
    span('a', 'root', 10, 50),
    span('a1', 'a', 20, 30),
    span('b', 'root', 60, 100, 'payment'),
  ])

  test('rows know whether they have children', () => {
    expect(rows.map((r) => [r.span.span_id, r.hasChildren])).toEqual([
      ['root', true], ['a', true], ['a1', false], ['b', false],
    ])
  })

  test('collapsing a span hides its descendants only', () => {
    const ids = visibleRows(rows, new Set(['a']), '').map((r) => r.span.span_id)
    expect(ids).toEqual(['root', 'a', 'b'])
  })

  test('search keeps matching spans and their ancestors', () => {
    const ids = visibleRows(rows, new Set(), 'payment').map((r) => r.span.span_id)
    expect(ids).toEqual(['root', 'b'])
    expect(visibleRows(rows, new Set(), 'A1').map((r) => r.span.span_id)).toEqual(['root', 'a', 'a1'])
  })
})
