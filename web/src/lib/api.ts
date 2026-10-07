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

export interface TimelinePoint {
  t: number
  requests: number
  errors: number
  p50_ms: number
  p95_ms: number
  p99_ms: number
}

export interface Operation {
  name: string
  requests: number
  errors: number
  p50_ms: number
  p95_ms: number
}

export interface Dependency {
  service: string
  requests: number
  errors: number
}

export interface ServiceDetail {
  summary: ServiceSummary
  step: number
  timeline: TimelinePoint[]
  operations: Operation[]
  calls: Dependency[]
  called_by: Dependency[]
}

export interface Edge {
  from: string
  to: string
  requests: number
  errors: number
}

export interface AlertRule {
  id?: string
  name: string
  kind: 'logs' | 'metric'
  metric?: string
  agg?: string
  filters?: Record<string, string>
  query?: string
  group_by?: string[]
  op: '>' | '>=' | '<' | '<='
  threshold: number
  window_seconds: number
  for_seconds: number
  channels: string[]
  enabled: boolean
  created_at?: string
  updated_at?: string
}

export interface AlertState {
  rule_id: string
  group: string
  labels: Record<string, string>
  status: 'ok' | 'pending' | 'firing'
  since: string
  value: number
}

export interface RuleStatus extends AlertRule {
  id: string
  status: 'ok' | 'pending' | 'firing'
  firing: number
  states: AlertState[]
}

export interface Channel {
  id?: string
  name: string
  type: 'webhook' | 'slack'
  url: string
}

export interface AlertEvent {
  id: number
  rule_id: string
  rule_name: string
  labels: Record<string, string>
  status: 'firing' | 'resolved'
  value: number
  at: string
}

export interface PreviewValue {
  labels: Record<string, string>
  value: number
  breached: boolean
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

async function send<T>(method: 'POST' | 'PUT' | 'DELETE', path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (res.status === 204) return undefined as T
  const json = (await res.json().catch(() => ({}))) as { data?: T; error?: string }
  if (!res.ok) throw new ApiError(json.error ?? res.statusText, res.status)
  return json.data as T
}

export const api = {
  services: (range: TimeRange) => get<ServiceSummary[]>('/api/v1/services', range),

  service: (range: TimeRange, name: string) =>
    get<ServiceDetail>(`/api/v1/services/${encodeURIComponent(name)}`, range),

  serviceMap: (range: TimeRange) => get<Edge[]>('/api/v1/service-map', range),

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

  alertRules: () => get<RuleStatus[]>('/api/v1/alerts/rules', null),
  createRule: (r: AlertRule) => send<AlertRule>('POST', '/api/v1/alerts/rules', r),
  updateRule: (id: string, r: AlertRule) => send<AlertRule>('PUT', `/api/v1/alerts/rules/${encodeURIComponent(id)}`, r),
  deleteRule: (id: string) => send<void>('DELETE', `/api/v1/alerts/rules/${encodeURIComponent(id)}`),
  previewRule: (r: AlertRule) => send<PreviewValue[]>('POST', '/api/v1/alerts/preview', r),
  channels: () => get<Channel[]>('/api/v1/alerts/channels', null),
  createChannel: (c: Channel) => send<Channel>('POST', '/api/v1/alerts/channels', c),
  deleteChannel: (id: string) => send<void>('DELETE', `/api/v1/alerts/channels/${encodeURIComponent(id)}`),
  testChannel: (id: string) => send<{ result: string }>('POST', `/api/v1/alerts/channels/${encodeURIComponent(id)}/test`),
  alertEvents: () => get<AlertEvent[]>('/api/v1/alerts/events', null),

  queryMetric: (range: TimeRange, p: MetricParams) =>
    get<Series[]>('/api/v1/metrics/query', range, [
      ['metric', p.metric],
      ['agg', p.agg],
      ['group_by', p.groupBy],
      ['filter', p.filters],
      ['step', p.step],
    ]),
}
