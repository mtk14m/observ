<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { SECTIONS } from '@/sections'
import { useTheme } from '@/composables/useTheme'
import AppIcon from './AppIcon.vue'

const route = useRoute()
const { theme, toggle } = useTheme()

// Navigation keeps the global time range, never page-specific filters.
const timeQuery = computed(() => {
  const { from, to } = route.query
  return typeof from === 'string' && typeof to === 'string' ? { from, to } : {}
})

const groups = computed(() => {
  const out: (typeof SECTIONS)[number][][] = []
  for (const s of SECTIONS) (out[s.group] ??= []).push(s)
  return out
})
</script>

<template>
  <aside class="rail">
    <RouterLink to="/" class="logo" aria-label="obsrv home">
      <svg viewBox="0 0 64 64" width="26" height="26" aria-hidden="true">
        <g fill="none" stroke="var(--accent)" stroke-width="6">
          <circle cx="32" cy="32" r="25" />
          <circle cx="32" cy="32" r="12" />
        </g>
        <circle cx="32" cy="32" r="4" fill="var(--accent)" />
      </svg>
    </RouterLink>

    <nav aria-label="Main">
      <div v-for="(group, i) in groups" :key="i" class="group">
        <RouterLink
          v-for="section in group"
          :key="section.path"
          :to="{ path: section.path, query: timeQuery }"
          :aria-label="section.name"
          :data-label="section.name"
          class="item"
          :class="{ active: route.path.startsWith(section.path) }"
        >
          <AppIcon :name="section.icon" :size="20" />
        </RouterLink>
      </div>
    </nav>

    <button
      type="button"
      class="item theme"
      :aria-label="theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'"
      :data-label="theme === 'dark' ? 'Light theme' : 'Dark theme'"
      @click="toggle"
    >
      <AppIcon :name="theme === 'dark' ? 'sun' : 'moon'" :size="20" />
    </button>
  </aside>
</template>

<style scoped>
.rail {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: var(--rail-width);
  padding: var(--space-3) 0;
  background: var(--bg-rail);
  border-right: 1px solid var(--border);
}
.logo {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  margin-bottom: var(--space-4);
}
nav {
  display: flex;
  flex-direction: column;
  flex: 1;
  gap: var(--space-5);
}
.group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.item {
  position: relative;
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border: 0;
  border-radius: var(--radius);
  background: none;
  color: var(--text-muted);
  cursor: pointer;
}
.item:hover {
  background: var(--bg-hover);
  color: var(--text);
}
.item.active {
  background: var(--accent-soft);
  color: var(--accent-text);
}
/* Label tooltip on hover and keyboard focus. */
.item::after {
  content: attr(data-label);
  position: absolute;
  left: calc(100% + 10px);
  top: 50%;
  z-index: 30;
  padding: 5px 9px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg-elevated);
  box-shadow: var(--shadow);
  color: var(--text);
  font-size: 13px;
  white-space: nowrap;
  opacity: 0;
  pointer-events: none;
  transform: translate(-4px, -50%);
  transition: opacity 100ms, transform 100ms;
}
.item:hover::after,
.item:focus-visible::after {
  opacity: 1;
  transform: translate(0, -50%);
}
</style>
