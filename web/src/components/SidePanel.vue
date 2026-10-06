<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue'

defineProps<{ title: string }>()
const emit = defineEmits<{ close: [] }>()

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}
onMounted(() => document.addEventListener('keydown', onKey))
onBeforeUnmount(() => document.removeEventListener('keydown', onKey))
</script>

<template>
  <aside class="panel" role="dialog" :aria-label="title">
    <header>
      <h2>{{ title }}</h2>
      <button type="button" class="close" aria-label="Close panel" @click="emit('close')">✕</button>
    </header>
    <div class="body">
      <slot />
    </div>
  </aside>
</template>

<style scoped>
.panel {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 20;
  display: flex;
  flex-direction: column;
  width: min(560px, 92vw);
  background: var(--bg-elevated);
  border-left: 1px solid var(--border);
  box-shadow: var(--shadow);
}
header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  height: 48px;
  padding: 0 var(--space-4);
  border-bottom: 1px solid var(--border);
}
h2 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.close {
  border: 0;
  background: none;
  color: var(--text-muted);
  font-size: 14px;
  cursor: pointer;
}
.close:hover {
  color: var(--text);
}
.body {
  flex: 1;
  overflow: auto;
  padding: var(--space-4);
}
</style>
