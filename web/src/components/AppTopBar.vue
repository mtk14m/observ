<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import type { IconName } from '@/sections'
import AppIcon from './AppIcon.vue'
import TimeRangePicker from './TimeRangePicker.vue'

interface Meta {
  title?: string
  icon?: IconName
  parent?: { title: string; path: string }
  /** Route param shown as the title of detail pages. */
  param?: string
  /** Shorten the param (trace IDs). */
  short?: boolean
  timeless?: boolean
}

const route = useRoute()
const meta = computed(() => route.meta as Meta)
const title = computed(() => {
  const m = meta.value
  if (m.param) {
    const v = String(route.params[m.param] ?? '')
    return m.short ? v.slice(0, 8) : v
  }
  return m.title ?? ''
})
const timeQuery = computed(() => {
  const { from, to } = route.query
  return typeof from === 'string' && typeof to === 'string' ? { from, to } : {}
})
</script>

<template>
  <header class="topbar">
    <div class="heading">
      <AppIcon v-if="meta.icon" :name="meta.icon" :size="20" class="page-icon" />
      <template v-if="meta.parent">
        <RouterLink :to="{ path: meta.parent.path, query: timeQuery }" class="crumb">{{ meta.parent.title }}</RouterLink>
        <span class="sep" aria-hidden="true">/</span>
      </template>
      <h1>{{ title }}</h1>
    </div>
    <TimeRangePicker v-if="!meta.timeless" />
  </header>
</template>

<style scoped>
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 56px;
  padding: 0 var(--space-5);
  border-bottom: 1px solid var(--border);
  background: var(--bg);
}
.heading {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.page-icon {
  color: var(--text-muted);
}
.crumb {
  color: var(--text-muted);
  font-size: 17px;
}
.crumb:hover {
  color: var(--text);
}
.sep {
  color: var(--text-faint);
  font-size: 17px;
}
h1 {
  margin: 0;
  font-size: 17px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
