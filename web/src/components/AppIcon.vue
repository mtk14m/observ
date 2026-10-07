<script setup lang="ts">
import type { IconName } from '@/sections'

export type UtilityIcon =
  | 'chevron' | 'clock' | 'search' | 'close' | 'sun' | 'moon' | 'external' | 'expand' | 'collapse'
  | 'filter' | 'plus' | 'arrow-left'

defineProps<{ name: IconName | UtilityIcon; size?: number }>()

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
  search: 'M11 18a7 7 0 100-14 7 7 0 000 14zm9 2l-4-4',
  close: 'M6 6l12 12M18 6L6 18',
  sun: 'M12 16a4 4 0 100-8 4 4 0 000 8zM12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4l1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4',
  moon: 'M20 14.5A8 8 0 019.5 4a8 8 0 1010.5 10.5z',
  external: 'M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 01-1 1H5a1 1 0 01-1-1V7a1 1 0 011-1h5',
  expand: 'M5 5h14v14H5zM9 12h6M12 9v6',
  collapse: 'M5 5h14v14H5zM9 12h6',
  filter: 'M4 6h16M7 12h10M10 18h4',
  plus: 'M12 5v14M5 12h14',
  'arrow-left': 'M19 12H5m6-6l-6 6 6 6',
}
</script>

<template>
  <svg
    class="icon"
    :width="size ?? 18"
    :height="size ?? 18"
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
  flex-shrink: 0;
}
</style>
