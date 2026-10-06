import { resolve, type TimeRange } from '@/lib/timeRange'

/** Bounds of a range in unix nanoseconds, resolved at call time. */
export function resolveNs(range: TimeRange): { from: number; to: number } {
  const r = resolve(range, new Date())
  if (!r) return { from: 0, to: 0 }
  return { from: r.from.getTime() * 1e6, to: r.to.getTime() * 1e6 }
}
