import { afterEach, beforeEach, expect, test, vi } from 'vitest'
import { useTheme } from './useTheme'

function prefersDark(dark: boolean) {
  vi.stubGlobal('matchMedia', (q: string) => ({ matches: q.includes('dark') ? dark : !dark, media: q }))
}

beforeEach(() => {
  localStorage.clear()
  delete document.documentElement.dataset.theme
})
afterEach(() => vi.unstubAllGlobals())

test('follows the system until the user chooses', () => {
  prefersDark(false)
  const t = useTheme()
  expect(t.theme.value).toBe('light')
  expect(document.documentElement.dataset.theme).toBeUndefined()
})

test('toggling forces the other theme and remembers it', () => {
  prefersDark(true)
  const t = useTheme()
  t.toggle()
  expect(t.theme.value).toBe('light')
  expect(document.documentElement.dataset.theme).toBe('light')
  expect(localStorage.getItem('obsrv.theme')).toBe('light')

  // A new page load restores the choice.
  delete document.documentElement.dataset.theme
  const again = useTheme()
  expect(again.theme.value).toBe('light')
  expect(document.documentElement.dataset.theme).toBe('light')
})

test('works when storage is unavailable', () => {
  prefersDark(true)
  vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
    throw new Error('blocked')
  })
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
    throw new Error('blocked')
  })
  const t = useTheme()
  expect(t.theme.value).toBe('dark')
  t.toggle()
  expect(t.theme.value).toBe('light')
})
