<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { SECTIONS } from '@/sections'
import AppIcon from './AppIcon.vue'

const route = useRoute()
const expanded = ref(true)

// Navigation keeps the global time range, never page-specific filters.
const timeQuery = computed(() => {
  const { from, to } = route.query
  return typeof from === 'string' && typeof to === 'string' ? { from, to } : {}
})
</script>

<template>
  <aside class="sidebar" :class="{ collapsed: !expanded }">
    <div class="brand">
      <span class="logo" aria-hidden="true">◎</span>
      <span class="label">obsrv</span>
    </div>

    <nav id="sidebar-nav" aria-label="Main">
      <RouterLink
        v-for="section in SECTIONS"
        :key="section.path"
        :to="{ path: section.path, query: timeQuery }"
        :aria-label="section.name"
        :title="expanded ? undefined : section.name"
        class="item"
      >
        <AppIcon :name="section.icon" />
        <span class="label">{{ section.name }}</span>
      </RouterLink>
    </nav>

    <button
      class="toggle item"
      type="button"
      aria-controls="sidebar-nav"
      :aria-expanded="expanded"
      :aria-label="expanded ? 'Collapse sidebar' : 'Expand sidebar'"
      @click="expanded = !expanded"
    >
      <AppIcon name="panel" />
      <span class="label">Collapse</span>
    </button>
  </aside>
</template>

<style scoped>
.sidebar {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  width: 200px;
  padding: var(--space-2);
  background: var(--bg-sidebar);
  border-right: 1px solid var(--border);
  transition: width 120ms ease;
}
.sidebar.collapsed {
  width: 52px;
}
.collapsed .label {
  display: none;
}
.brand {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  height: 36px;
  padding: 0 var(--space-2);
  margin-bottom: var(--space-2);
  font-weight: 650;
  font-size: 15px;
  letter-spacing: -0.01em;
}
.logo {
  color: var(--accent);
  font-size: 18px;
}
nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}
.item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  height: 32px;
  padding: 0 var(--space-2);
  border-radius: var(--radius);
  color: var(--text-muted);
  white-space: nowrap;
}
.item:hover {
  background: var(--bg-hover);
  color: var(--text);
}
.item[aria-current='page'] {
  background: var(--accent-soft);
  color: var(--text);
}
.item[aria-current='page'] .icon {
  color: var(--accent);
}
.toggle {
  border: 0;
  background: none;
  font: inherit;
  cursor: pointer;
}
</style>
