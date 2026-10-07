<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import AppIcon from './AppIcon.vue'
import { formatCompact } from '@/lib/format'

const props = defineProps<{ label: string; facetKey: string; load: () => Promise<{ value: string; count: number }[]> }>()
const emit = defineEmits<{ select: [value: string] }>()

const open = ref(false)
const loading = ref(false)
const values = ref<{ value: string; count: number }[]>([])
const error = ref('')
const root = ref<HTMLElement>()

async function toggle() {
  open.value = !open.value
  if (!open.value) return
  loading.value = true
  error.value = ''
  try {
    values.value = await props.load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

function pick(v: string) {
  open.value = false
  emit('select', v)
}

function onDocumentClick(e: MouseEvent) {
  if (open.value && !root.value?.contains(e.target as Node)) open.value = false
}
onMounted(() => document.addEventListener('click', onDocumentClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick))
</script>

<template>
  <div ref="root" class="facet-wrap">
    <button type="button" class="chip facet" :data-key="facetKey" :aria-expanded="open" @click="toggle">
      {{ label }} <AppIcon name="chevron" :size="14" />
    </button>
    <ul v-if="open" class="facet-menu" role="listbox" :aria-label="label" @keydown.escape="open = false">
      <li v-if="loading" class="muted">Loading…</li>
      <li v-else-if="error" class="muted">{{ error }}</li>
      <li v-else-if="!values.length" class="muted">No values in this range</li>
      <li v-for="v in values" :key="v.value" role="option" tabindex="0" @click="pick(v.value)" @keydown.enter="pick(v.value)">
        <span class="value">{{ v.value || '(empty)' }}</span>
        <span class="n">{{ formatCompact(v.count) }}</span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.facet-wrap {
  position: relative;
}
.facet-menu {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  z-index: 15;
  min-width: 220px;
  max-height: 320px;
  margin: 0;
  padding: var(--space-1);
  overflow: auto;
  list-style: none;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
}
.facet-menu li {
  display: flex;
  justify-content: space-between;
  gap: var(--space-4);
  padding: 7px 10px;
  border-radius: var(--radius-sm);
  font-size: 13px;
}
[role='option'] {
  cursor: pointer;
}
[role='option']:hover,
[role='option']:focus {
  background: var(--bg-hover);
  outline: none;
}
.value {
  font-family: var(--font-mono);
}
.n {
  color: var(--text-muted);
  font-variant-numeric: tabular-nums;
}
</style>
