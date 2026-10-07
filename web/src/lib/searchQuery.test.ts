import { describe, expect, test } from 'vitest'
import { addFilter, filters, removeFilter } from './searchQuery'

describe('filters', () => {
  test('extracts key:value terms, leaving free text out', () => {
    expect(filters('service:api level:error "time out" -env:dev oops')).toEqual([
      { raw: 'service:api', key: 'service', value: 'api', negated: false },
      { raw: 'level:error', key: 'level', value: 'error', negated: false },
      { raw: '-env:dev', key: 'env', value: 'dev', negated: true },
    ])
  })

  test('handles quoted values and URLs', () => {
    expect(filters('user.name:"Ada L" url.full:http://x:1/a')).toEqual([
      { raw: 'user.name:"Ada L"', key: 'user.name', value: 'Ada L', negated: false },
      { raw: 'url.full:http://x:1/a', key: 'url.full', value: 'http://x:1/a', negated: false },
    ])
  })
})

test('addFilter appends a term, quoting values with spaces, without duplicates', () => {
  expect(addFilter('', 'level', 'error')).toBe('level:error')
  expect(addFilter('timeout', 'service', 'checkout')).toBe('timeout service:checkout')
  expect(addFilter('a', 'user.name', 'Ada L')).toBe('a user.name:"Ada L"')
  expect(addFilter('level:error', 'level', 'error')).toBe('level:error')
})

test('removeFilter removes exactly one term', () => {
  expect(removeFilter('service:api level:error timeout', 'level:error')).toBe('service:api timeout')
  expect(removeFilter('level:error', 'level:error')).toBe('')
})
