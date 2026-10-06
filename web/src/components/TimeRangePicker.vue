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
        {{ preset.label }}
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
  height: 30px;
  padding: 0 var(--space-2) 0 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-elevated);
  color: var(--text);
  font: inherit;
  cursor: pointer;
}
.trigger:hover {
  border-color: var(--border-strong);
}
.menu {
  position: absolute;
  right: 0;
  top: calc(100% + 4px);
  z-index: 10;
  min-width: 180px;
  margin: 0;
  padding: var(--space-1);
  list-style: none;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
}
[role='option'] {
  padding: 6px var(--space-2);
  border-radius: calc(var(--radius) - 2px);
  cursor: pointer;
}
[role='option']:hover {
  background: var(--bg-hover);
}
[role='option'][aria-selected='true'] {
  color: var(--accent);
}
</style>
