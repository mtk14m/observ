<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/lib/api'
import { paletteItems, shiftRange } from '@/lib/palette'
import { useTimeRange } from '@/composables/useTimeRange'
import AppIcon from './AppIcon.vue'

const router = useRouter()
const { range, setRange } = useTimeRange()

const open = ref(false)
const text = ref('')
const index = ref(0)
const input = ref<HTMLInputElement>()
const services = ref<string[]>([])
const metrics = ref<string[]>([])

const items = computed(() => paletteItems(text.value, { services: services.value, metrics: metrics.value }))
watch(text, () => (index.value = 0))

async function show() {
  open.value = true
  text.value = ''
  index.value = 0
  await nextTick()
  input.value?.focus()
  // Names for suggestions; failures only mean fewer suggestions.
  api.services(range.value).then((s) => (services.value = s.map((x) => x.name)), () => undefined)
  api.metrics(range.value).then((m) => (metrics.value = m.map((x) => x.name)), () => undefined)
}

async function go(i = index.value) {
  const item = items.value[i]
  open.value = false
  if (item) await router.push(item.to)
}

function onInputKey(e: KeyboardEvent) {
  if (e.key === 'ArrowDown') index.value = Math.min(index.value + 1, items.value.length - 1)
  else if (e.key === 'ArrowUp') index.value = Math.max(index.value - 1, 0)
  else if (e.key === 'Enter') void go()
  else if (e.key === 'Escape') open.value = false
  else return
  e.preventDefault()
}

function typing(t: EventTarget | null) {
  const el = t as HTMLElement | null
  return !!el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT' || el.isContentEditable)
}

// Global shortcuts.
function onKey(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault()
    if (open.value) open.value = false
    else void show()
    return
  }
  if (open.value || typing(e.target) || e.metaKey || e.ctrlKey || e.altKey) return
  if (e.key === '/') {
    const search = document.querySelector<HTMLElement>('[data-search]')
    if (search) {
      e.preventDefault()
      search.focus()
    }
  } else if (e.key === '[' || e.key === ']') {
    const next = shiftRange(range.value, e.key === '[' ? -1 : 1, new Date())
    if (next) void setRange(next)
  }
}
onMounted(() => document.addEventListener('keydown', onKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey))
</script>

<template>
  <div v-if="open" class="backdrop" @click.self="open = false">
    <div class="palette" role="dialog" aria-label="Command palette">
      <label class="field">
        <AppIcon name="search" :size="18" />
        <input
          ref="input"
          v-model="text"
          placeholder="Go to a page, a service, a metric, or paste a trace ID…"
          aria-label="Command"
          @keydown="onInputKey"
        />
        <kbd>esc</kbd>
      </label>
      <ul role="listbox">
        <li
          v-for="(item, i) in items"
          :key="item.label"
          role="option"
          :aria-selected="i === index"
          @mouseenter="index = i"
          @click="go(i)"
        >
          {{ item.label }}
        </li>
      </ul>
      <footer class="muted">
        <span><kbd>↑</kbd><kbd>↓</kbd> move</span>
        <span><kbd>↵</kbd> open</span>
        <span><kbd>/</kbd> search page</span>
        <span><kbd>[</kbd><kbd>]</kbd> shift period</span>
      </footer>
    </div>
  </div>
</template>

<style scoped>
.backdrop {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  justify-content: center;
  padding-top: 14vh;
  background: rgb(0 0 0 / 35%);
}
.palette {
  width: min(620px, 92vw);
  max-height: 60vh;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--bg-elevated);
  box-shadow: var(--shadow);
  overflow: hidden;
}
.field {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: 0 var(--space-4);
  border-bottom: 1px solid var(--border);
  color: var(--text-muted);
}
.field input {
  flex: 1;
  height: 52px;
  border: 0;
  background: none;
  color: var(--text);
  font: inherit;
  font-size: 16px;
  outline: none;
}
ul {
  flex: 1;
  margin: 0;
  padding: var(--space-1);
  overflow: auto;
  list-style: none;
}
[role='option'] {
  padding: 9px 12px;
  border-radius: var(--radius-sm);
  cursor: pointer;
}
[role='option'][aria-selected='true'] {
  background: var(--accent-soft);
  color: var(--accent-text);
}
footer {
  display: flex;
  gap: var(--space-4);
  padding: 8px var(--space-4);
  border-top: 1px solid var(--border);
  font-size: 12px;
}
kbd {
  display: inline-block;
  min-width: 18px;
  margin-right: 3px;
  padding: 0 4px;
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  font-family: var(--font-mono);
  font-size: 11px;
  text-align: center;
}
</style>
