<script setup lang="ts">
import { computed } from 'vue'

/** A tiny bar chart of counts, to show a trend inline. */
const props = withDefaults(defineProps<{ values: number[]; width?: number; height?: number }>(), { width: 96, height: 22 })
const max = computed(() => Math.max(1, ...props.values))
const barW = computed(() => props.width / Math.max(props.values.length, 1))
</script>

<template>
  <svg class="spark" :width="width" :height="height" :viewBox="`0 0 ${width} ${height}`" aria-hidden="true">
    <rect
      v-for="(v, i) in values"
      :key="i"
      :x="i * barW + 0.5"
      :y="height - Math.max(v > 0 ? 2 : 0, (v / max) * height)"
      :width="Math.max(barW - 1, 1)"
      :height="Math.max(v > 0 ? 2 : 0, (v / max) * height)"
      rx="1"
    />
  </svg>
</template>

<style scoped>
.spark rect {
  fill: var(--level-error);
  opacity: 0.85;
}
</style>
