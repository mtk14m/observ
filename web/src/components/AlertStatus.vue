<script setup lang="ts">
defineProps<{ status: 'ok' | 'pending' | 'firing' | 'resolved' }>()
const icons = { firing: '⚠', pending: '◔', ok: '✓', resolved: '✓' }
</script>

<template>
  <span class="status" :class="status"><span aria-hidden="true">{{ icons[status] }}</span> {{ status.toUpperCase() }}</span>
</template>

<style scoped>
.status {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 1px 6px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.03em;
}
.firing {
  color: var(--status-error);
  background: color-mix(in srgb, var(--status-error) 14%, transparent);
}
.pending {
  color: var(--status-warn);
  background: color-mix(in srgb, var(--status-warn) 14%, transparent);
}
.ok,
.resolved {
  color: var(--status-ok);
  background: color-mix(in srgb, var(--status-ok) 12%, transparent);
}
</style>
