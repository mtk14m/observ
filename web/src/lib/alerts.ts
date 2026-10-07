import type { AlertRule } from './api'

export function formatWindow(seconds: number): string {
  if (seconds % 3600 === 0) return `${seconds / 3600}h`
  if (seconds % 60 === 0) return `${seconds / 60}m`
  return `${seconds}s`
}

/** Describes a rule's condition in plain words. */
export function describeCondition(r: AlertRule): string {
  const subject =
    r.kind === 'logs'
      ? r.query
        ? `count of logs matching ${r.query}`
        : 'count of logs'
      : `${r.agg || 'value'} of ${r.metric}`
  let s = `${subject} ${r.op} ${r.threshold} over ${formatWindow(r.window_seconds)}`
  if (r.group_by?.length) s += `, per ${r.group_by.join(', ')}`
  if (r.for_seconds > 0) s += `, for ${formatWindow(r.for_seconds)}`
  return s
}

/** A readable name for an alert group. */
export function groupLabel(labels: Record<string, string>): string {
  const entries = Object.entries(labels).sort(([a], [b]) => a.localeCompare(b))
  return entries.length ? entries.map(([k, v]) => `${k}=${v}`).join(' · ') : 'all'
}
