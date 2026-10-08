<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { closeValueMenu, valueMenu } from '@/composables/useValueMenu'
import { valueActions, type ValueAction } from '@/lib/valueActions'

const route = useRoute()
const router = useRouter()
const root = ref<HTMLElement>()

const actions = computed(() =>
  valueMenu.open ? valueActions({ key: valueMenu.key, value: valueMenu.value }, { path: route.path, query: route.query }) : [],
)
const style = computed(() => ({
  left: `${Math.min(valueMenu.x, window.innerWidth - 280)}px`,
  top: `${Math.min(valueMenu.y, window.innerHeight - 260)}px`,
}))

async function run(a: ValueAction) {
  closeValueMenu()
  if (a.copy !== undefined) await navigator.clipboard?.writeText(a.copy)
  if (a.to) await router.push(a.to)
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') closeValueMenu()
}
function onClick(e: MouseEvent) {
  const t = e.target as HTMLElement
  if (valueMenu.open && !root.value?.contains(t) && !t.closest('.val')) closeValueMenu()
}
onMounted(() => {
  document.addEventListener('keydown', onKey)
  document.addEventListener('click', onClick)
})
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKey)
  document.removeEventListener('click', onClick)
})
</script>

<template>
  <div v-if="valueMenu.open" ref="root" class="value-menu" role="menu" :style="style">
    <div class="head">
      <span class="k">{{ valueMenu.key }}</span>
      <span class="v">{{ valueMenu.value }}</span>
    </div>
    <button v-for="a in actions" :key="a.label" type="button" role="menuitem" class="item" @click="run(a)">
      {{ a.label }}
    </button>
  </div>
</template>

<style scoped>
.value-menu {
  position: fixed;
  z-index: 50;
  min-width: 240px;
  max-width: 360px;
  padding: var(--space-1);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-elevated);
  box-shadow: var(--shadow);
}
.head {
  display: flex;
  flex-direction: column;
  padding: 8px 10px;
  margin-bottom: var(--space-1);
  border-bottom: 1px solid var(--border);
  font-family: var(--font-mono);
  font-size: 12px;
}
.k {
  color: var(--text-muted);
}
.v {
  overflow-wrap: anywhere;
}
.item {
  display: block;
  width: 100%;
  padding: 7px 10px;
  border: 0;
  border-radius: var(--radius-sm);
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.item:hover,
.item:focus-visible {
  background: var(--bg-hover);
  outline: none;
}
</style>
