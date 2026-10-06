/** Small scale helpers shared by the SVG charts. */

/** Rounds max up to a "nice" number so axis ticks are readable. */
export function niceMax(max: number): number {
  if (max <= 0 || !Number.isFinite(max)) return 1
  const exp = 10 ** Math.floor(Math.log10(max))
  const f = max / exp
  const nice = f <= 1 ? 1 : f <= 2 ? 2 : f <= 2.5 ? 2.5 : f <= 5 ? 5 : 10
  return nice * exp
}

/** Evenly spaced time ticks (unix ns) at round minute/hour boundaries. */
export function timeTicks(from: number, to: number, count: number): number[] {
  const spanMs = (to - from) / 1e6
  const steps = [1, 5, 10, 15, 30, 60, 120, 300, 600, 900, 1800, 3600, 7200, 21600, 43200, 86400].map((s) => s * 1000)
  const step = steps.find((s) => spanMs / s <= count) ?? steps[steps.length - 1]!
  const first = Math.ceil(from / 1e6 / step) * step
  const ticks: number[] = []
  for (let t = first; t * 1e6 <= to; t += step) ticks.push(t * 1e6)
  return ticks
}

export function tickLabel(ns: number, spanNs: number): string {
  const d = new Date(ns / 1e6)
  const hm = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  if (spanNs > 2 * 86400e9) return `${d.getMonth() + 1}/${d.getDate()}`
  if (spanNs < 120e9) return `${hm}:${String(d.getSeconds()).padStart(2, '0')}`
  return hm
}
