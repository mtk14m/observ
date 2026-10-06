<script setup lang="ts">
defineProps<{ label: string; value: string; status?: 'error' }>()
</script>

<template>
  <div class="tile">
    <div class="label">{{ label }}</div>
    <div class="value" :class="status"><span v-if="status === 'error'" aria-hidden="true">⚠ </span>{{ value }}</div>
  </div>
</template>

<style scoped>
.tile {
  padding: var(--space-3) var(--space-4);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}
.label {
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.03em;
  text-transform: uppercase;
}
.value {
  margin-top: 2px;
  font-size: 22px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.value.error {
  color: var(--status-error);
}
</style>
