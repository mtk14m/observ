import type { MetricInfo } from './api'
import { formatValue } from './format'

export interface AggOption {
  value: string
  label: string
}

/** Aggregations that make sense for a metric, the default first. */
export function aggregations(m: Pick<MetricInfo, 'type'> & { monotonic?: boolean }): AggOption[] {
  switch (m.type) {
    case 'Histogram':
      return [
        { value: 'p95', label: 'p95' },
        { value: 'p50', label: 'p50' },
        { value: 'p90', label: 'p90' },
        { value: 'p99', label: 'p99' },
        { value: 'avg', label: 'average' },
        { value: 'count', label: 'count per second' },
      ]
    case 'ExponentialHistogram':
    case 'Summary':
      return [
        { value: 'avg', label: 'average' },
        { value: 'count', label: 'count per second' },
      ]
    default:
      return [
        { value: 'avg', label: 'average' },
        { value: 'sum', label: 'sum' },
        { value: 'max', label: 'max' },
        { value: 'min', label: 'min' },
      ]
  }
}

/** Formats values of a metric query result according to type, unit and aggregation. */
export function valueFormatter(m: MetricInfo, agg: string, isRate: boolean): (v: number) => string {
  if (agg === 'count') return (v) => `${formatValue(v, '')}/s`
  if (isRate) return (v) => `${formatValue(v, m.unit)}/s`
  return (v) => formatValue(v, m.unit)
}

/** Series label from its group-by values. */
export function seriesLabel(labels: Record<string, string>, metric: string): string {
  const values = Object.values(labels)
  if (values.length === 0) return metric
  return values.map((v) => v || '(none)').join(' · ')
}
