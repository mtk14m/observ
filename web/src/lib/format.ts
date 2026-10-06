/** Formatting helpers. All durations from the API are in nanoseconds. */

const sig = (v: number) => String(Number.parseFloat(v.toPrecision(3)))

export function formatDuration(ns: number): string {
  if (ns === 0) return '0 ms'
  if (ns < 1e3) return `${Math.round(ns)} ns`
  if (ns < 1e6) return `${sig(ns / 1e3)} µs`
  if (ns < 1e9) return `${sig(ns / 1e6)} ms`
  if (ns < 60e9) return `${sig(ns / 1e9)} s`
  const s = Math.round(ns / 1e9)
  return `${Math.floor(s / 60)}m ${s % 60}s`
}

export const formatMs = (ms: number) => formatDuration(ms * 1e6)

const pad = (n: number, w = 2) => String(n).padStart(w, '0')

/** Local wall-clock time with milliseconds, from unix nanoseconds. */
export function formatTime(ns: number): string {
  const d = new Date(ns / 1e6)
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}.${pad(d.getMilliseconds(), 3)}`
}

/** Local date and time, for headers. */
export function formatDateTime(ns: number): string {
  const d = new Date(ns / 1e6)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${formatTime(ns)}`
}

export function formatPercent(ratio: number): string {
  const p = ratio * 100
  if (p === 0) return '0%'
  return p < 10 ? `${Number.parseFloat(p.toFixed(1))}%` : `${Math.round(p)}%`
}

export const formatRate = (perSecond: number) => `${sig(perSecond)}/s`

export function formatCompact(v: number): string {
  const a = Math.abs(v)
  if (a >= 1e9) return `${sig(v / 1e9)}B`
  if (a >= 1e6) return `${sig(v / 1e6)}M`
  if (a >= 1e4) return `${sig(v / 1e3)}K`
  return sig(v)
}

/** Formats a metric value using its OpenTelemetry unit. */
export function formatValue(v: number, unit: string): string {
  switch (unit) {
    case 's':
      return formatDuration(v * 1e9)
    case 'ms':
      return formatMs(v)
    case 'By':
      return formatBytes(v)
    case '%':
      return `${sig(v)}%`
    case '1':
    case '':
      return formatCompact(v)
    default:
      return unit.startsWith('{') ? formatCompact(v) : `${formatCompact(v)} ${unit}`
  }
}

export function formatBytes(v: number): string {
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let i = 0
  while (Math.abs(v) >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${sig(v)} ${units[i]}`
}
