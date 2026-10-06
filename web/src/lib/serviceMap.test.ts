import { describe, expect, test } from 'vitest'
import { layoutMap } from './serviceMap'

const e = (from: string, to: string, requests = 1, errors = 0) => ({ from, to, requests, errors })

describe('layoutMap', () => {
  test('places callers left of the services they call', () => {
    const { nodes } = layoutMap([e('web', 'api'), e('api', 'db'), e('mobile', 'api')], ['web', 'api', 'db', 'mobile', 'cron'])
    const col = Object.fromEntries(nodes.map((n) => [n.name, n.column]))
    expect(col).toEqual({ web: 0, mobile: 0, cron: 0, api: 1, db: 2 })
  })

  test('survives cycles', () => {
    const { nodes } = layoutMap([e('a', 'b'), e('b', 'a'), e('b', 'c')], ['a', 'b', 'c'])
    expect(nodes).toHaveLength(3)
    expect(Math.max(...nodes.map((n) => n.column))).toBeLessThanOrEqual(2)
  })

  test('orders nodes within a column by name and gives them distinct rows', () => {
    const { nodes, columns, rows } = layoutMap([e('x', 'b'), e('x', 'a')], ['x', 'a', 'b'])
    const inCol1 = nodes.filter((n) => n.column === 1).map((n) => [n.name, n.row])
    expect(inCol1).toEqual([['a', 0], ['b', 1]])
    expect(columns).toBe(2)
    expect(rows).toBe(2)
  })
})
