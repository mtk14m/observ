import type { Edge } from './api'

export interface MapNode {
  name: string
  column: number
  row: number
}

/**
 * Lays services out in columns: a service sits one column right of its
 * deepest caller, so requests flow left to right. Cycles are broken by
 * bounding the depth by the number of services.
 */
export function layoutMap(edges: Edge[], services: string[]): { nodes: MapNode[]; columns: number; rows: number } {
  const names = [...new Set([...services, ...edges.flatMap((e) => [e.from, e.to])])]
  const depth = new Map(names.map((n) => [n, 0]))
  for (let i = 0; i < names.length; i++) {
    let changed = false
    for (const e of edges) {
      const d = Math.min(depth.get(e.from)! + 1, names.length - 1)
      if (e.from !== e.to && d > depth.get(e.to)!) {
        depth.set(e.to, d)
        changed = true
      }
    }
    if (!changed) break
  }
  const byColumn = new Map<number, string[]>()
  for (const n of names) {
    const c = depth.get(n)!
    byColumn.set(c, [...(byColumn.get(c) ?? []), n])
  }
  const nodes: MapNode[] = []
  let rows = 0
  for (const [column, list] of byColumn) {
    list.sort().forEach((name, row) => nodes.push({ name, column, row }))
    rows = Math.max(rows, list.length)
  }
  return { nodes, columns: byColumn.size, rows }
}
