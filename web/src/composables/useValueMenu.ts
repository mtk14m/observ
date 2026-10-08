import { reactive } from 'vue'

/** State of the single value menu of the UI. */
export const valueMenu = reactive({
  open: false,
  key: '',
  value: '',
  x: 0,
  y: 0,
})

export function openValueMenu(e: MouseEvent, key: string, value: string) {
  const target = e.currentTarget as HTMLElement | null
  const rect = target?.getBoundingClientRect()
  valueMenu.key = key
  valueMenu.value = value
  valueMenu.x = rect ? rect.left : e.clientX
  valueMenu.y = rect ? rect.bottom + 4 : e.clientY
  valueMenu.open = true
}

export function closeValueMenu() {
  valueMenu.open = false
}
