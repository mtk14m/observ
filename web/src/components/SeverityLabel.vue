<script setup lang="ts">
import { levelColor } from '@/lib/severity'

defineProps<{ level: string }>()
</script>

<template>
  <span class="sev"><span class="bar" :style="{ background: levelColor(level) }" aria-hidden="true" />{{ level || 'UNSET' }}</span>
</template>

<style scoped>
.sev {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 600;
}
.bar {
  width: 3px;
  height: 14px;
  border-radius: 2px;
}
</style>
