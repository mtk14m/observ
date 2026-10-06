<script setup lang="ts">
import type { IconName } from '@/sections'

defineProps<{ name: IconName | 'chevron' | 'clock' | 'panel' }>()

// 24×24 stroke icons drawn for obsrv.
const paths: Record<string, string> = {
  services: 'M12 3l8 4.5v9L12 21l-8-4.5v-9L12 3zm0 0v18m8-13.5L12 12 4 7.5',
  traces: 'M4 6h10M8 12h12M6 18h8',
  logs: 'M5 5h14M5 9.5h14M5 14h9M5 18.5h12',
  metrics: 'M4 19l5-7 4 4 7-10',
  dashboards: 'M4 4h7v9H4zM13 4h7v5h-7zM13 11h7v9h-7zM4 15h7v5H4z',
  alerts: 'M12 4a5 5 0 015 5v4l2 3H5l2-3V9a5 5 0 015-5zm-2 15a2 2 0 004 0',
  chevron: 'M7 10l5 5 5-5',
  clock: 'M12 7v5l3 2M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
  panel: 'M4 5h16v14H4zM9 5v14',
}
</script>

<template>
  <svg
    class="icon"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.75"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
  >
    <path :d="paths[name]" />
  </svg>
</template>

<style scoped>
.icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}
</style>
