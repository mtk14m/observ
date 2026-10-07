<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { PRESETS, label } from '@/lib/timeRange'
import { useTimeRange } from '@/composables/useTimeRange'
import AppIcon from './AppIcon.vue'

const { range, setRange } = useTimeRange()
const open = ref(false)
const root = ref<HTMLElement>()

async function choose(from: string) {
  open.value = false
  await setRange({ from, to: 'now' })
}

function onDocumentClick(event: MouseEvent) {
  if (open.value && !root.value?.contains(event.target as Node)) open.value = false
}

onMounted(() => document.addEventListener('click', onDocumentClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick))
</script>

<template>
  <div ref="root" class="picker">
    <button
      type="button"
      class="trigger"
      aria-haspopup="listbox"
      :aria-expanded="open"
      @click="open = !open"
    >
      <AppIcon name="clock" />
      <span>{{ label(range) }}</span>
      <AppIcon name="chevron" />
    </button>

    <ul
      v-if="open"
      class="menu"
      role="listbox"
      tabindex="-1"
      aria-label="Time range"
      @keydown.escape="open = false"
    >
      <li
        v-for="preset in PRESETS"
        :key="preset.from"
        role="option"
        :aria-selected="range.to === 'now' && range.from === preset.from"
        @click="choose(preset.from)"
      >
        <span>{{ preset.label }}</span>
        <span v-if="range.to === 'now' && range.from === preset.from" class="tick" aria-hidden="true">✓</span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.picker {
  position: relative;
}
.trigger {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  height: 36px;
  padding: 0 10px 0 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-elevated);
  color: var(--text);
  font: inherit;
  font-weight: 500;
  cursor: pointer;
}
.trigger:hover {
  border-color: var(--border-strong);
}
.trigger .icon:first-child {
  color: var(--text-muted);
}
.menu {
  position: absolute;
  right: 0;
  top: calc(100% + 6px);
  z-index: 10;
  min-width: 200px;
  margin: 0;
  padding: var(--space-1);
  list-style: none;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
}
[role='option'] {
  display: flex;
  justify-content: space-between;
  padding: 8px 10px;
  border-radius: var(--radius-sm);
  cursor: pointer;
}
[role='option']:hover {
  background: var(--bg-hover);
}
[role='option'][aria-selected='true'] {
  font-weight: 600;
}
.tick {
  color: var(--accent-text);
}
</style>
