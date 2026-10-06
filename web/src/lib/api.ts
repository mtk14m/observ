/** Typed client for the obsrv JSON API (/api/v1). */
import type { TimeRange } from './timeRange'

export interface ServiceSummary {
  name: string
  requests: number
  errors: number
  error_rate: number
  rate_per_second: number
  p50_ms: number
  p95_ms: number
  p99_ms: number
}

export interface LogRecord {
  time: number
  service: string
  severity: string
  severity_number: number
  body: string
  trace_id: string
  span_id: string
  attributes: Record<string, string>
  resource_attributes: Record<string, string>
}

export interface HistogramBucket {
  t: number
  counts: Record<string, number>
}

export interface TraceSummary {
  trace_id: string
  root_service: string
  root_name: string
  start: number
  duration_nano: number
  span_count: number
  error_count: number
  services: string[]
}

export interface SpanEvent {
  time_unix_nano: number
  name: string
  attributes: Record<string, string>
}

export interface Span {
  trace_id: string
  span_id: string
  parent_span_id: string
  name: string
  kind: string
  start: number
  end: number
  duration_nano: number
  status_code: string
  status_message: string
  service: string
  scope_name: string
  attributes: Record<string, string>
  resource_attributes: Record<string, string>
  events: SpanEvent[]
}

export interface MetricInfo {
  name: string
  type: string
  unit: string
  series: number
  attribute_keys: string[]
  /** Counters are queried as per-second rates. */
  monotonic: boolean
}

export interface Series {
  labels: Record<string, string>
  points: { t: number; v: number }[]
}

export interface TraceFilters {
  service?: string
  errors?: boolean
  minDurationMs?: number
}

export interface MetricParams {
  metric: string
  agg?: string
  groupBy?: string[]
  filters?: string[]
  step?: string
}

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

type Param = string | number | boolean | string[] | undefined

async function get<T>(path: string, range: TimeRange | null, params: [string, Param][] = []): Promise<T> {
  const search = new URLSearchParams()
  if (range) {
    search.append('from', range.from)
    search.append('to', range.to)
  }
  for (const [key, value] of params) {
    if (value === undefined || value === '' || value === false) continue
    if (Array.isArray(value)) {
      if (key === 'filter') value.forEach((v) => search.append(key, v))
      else if (value.length) search.append(key, value.join(','))
      continue
    }
    search.append(key, String(value))
  }
  const qs = search.toString()
  const res = await fetch(qs ? `${path}?${qs}` : path)
  const body = (await res.json().catch(() => ({}))) as { data?: T; error?: string }
  if (!res.ok) throw new ApiError(body.error ?? res.statusText, res.status)
  return body.data as T
}

export const api = {
  services: (range: TimeRange) => get<ServiceSummary[]>('/api/v1/services', range),

  logs: (range: TimeRange, q: string, limit?: number) =>
    get<LogRecord[]>('/api/v1/logs', range, [['q', q], ['limit', limit]]),

  logHistogram: (range: TimeRange, q: string) =>
    get<HistogramBucket[]>('/api/v1/logs/histogram', range, [['q', q]]),

  traces: (range: TimeRange, f: TraceFilters = {}) =>
    get<TraceSummary[]>('/api/v1/traces', range, [
      ['service', f.service],
      ['errors', f.errors],
      ['min_duration_ms', f.minDurationMs],
    ]),

  trace: (id: string) => get<Span[]>(`/api/v1/traces/${encodeURIComponent(id)}`, null),

  metrics: (range: TimeRange) => get<MetricInfo[]>('/api/v1/metrics', range),

  queryMetric: (range: TimeRange, p: MetricParams) =>
    get<Series[]>('/api/v1/metrics/query', range, [
      ['metric', p.metric],
      ['agg', p.agg],
      ['group_by', p.groupBy],
      ['filter', p.filters],
      ['step', p.step],
    ]),
}
