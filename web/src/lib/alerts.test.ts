import { describe, expect, test } from 'vitest'
import { describeCondition, formatWindow, groupLabel } from './alerts'
import type { AlertRule } from './api'

const base: AlertRule = {
  name: 'r', kind: 'logs', query: 'service:checkout level:error', op: '>', threshold: 5,
  window_seconds: 300, for_seconds: 0, channels: [], enabled: true,
}

describe('describeCondition', () => {
  test('logs rules', () => {
    expect(describeCondition(base)).toBe('count of logs matching service:checkout level:error > 5 over 5m')
    expect(describeCondition({ ...base, query: '' })).toBe('count of logs > 5 over 5m')
  })

  test('metric rules with grouping and a wait period', () => {
    const r: AlertRule = {
      ...base, kind: 'metric', metric: 'http.server.request.duration', agg: 'p95', query: undefined,
      threshold: 0.5, group_by: ['service.name'], for_seconds: 120,
    }
    expect(describeCondition(r)).toBe('p95 of http.server.request.duration > 0.5 over 5m, per service.name, for 2m')
  })
})

test('formatWindow', () => {
  expect(formatWindow(30)).toBe('30s')
  expect(formatWindow(300)).toBe('5m')
  expect(formatWindow(3600)).toBe('1h')
  expect(formatWindow(5400)).toBe('90m')
})

test('groupLabel', () => {
  expect(groupLabel({})).toBe('all')
  expect(groupLabel({ 'service.name': 'checkout', env: 'prod' })).toBe('env=prod · service.name=checkout')
})
