import { ref } from 'vue'

type Theme = 'light' | 'dark'
const KEY = 'obsrv.theme'

function stored(): Theme | null {
  try {
    const v = localStorage.getItem(KEY)
    return v === 'light' || v === 'dark' ? v : null
  } catch {
    return null
  }
}

function systemTheme(): Theme {
  return typeof matchMedia === 'function' && matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark'
}

/**
 * The color theme. It follows the operating system until the user picks
 * one, which is then remembered in this browser.
 */
export function useTheme() {
  const choice = stored()
  if (choice) document.documentElement.dataset.theme = choice
  const theme = ref<Theme>(choice ?? systemTheme())

  function toggle() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    document.documentElement.dataset.theme = theme.value
    try {
      localStorage.setItem(KEY, theme.value)
    } catch {
      // Storage blocked: the choice lasts for this page only.
    }
  }

  return { theme, toggle }
}
