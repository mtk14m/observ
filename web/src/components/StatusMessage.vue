<script setup lang="ts">
defineProps<{ kind: 'empty' | 'error'; title: string; detail?: string }>()
</script>

<template>
  <div class="status" :class="kind" :role="kind === 'error' ? 'alert' : undefined">
    <strong><span v-if="kind === 'error'" aria-hidden="true">⚠ </span>{{ title }}</strong>
    <p v-if="detail">{{ detail }}</p>
    <slot />
  </div>
</template>

<style scoped>
.status {
  padding: var(--space-6) var(--space-4);
  text-align: center;
  color: var(--text-muted);
}
.status strong {
  display: block;
  color: var(--text);
  font-weight: 600;
}
.status p {
  margin: var(--space-1) 0 0;
}
.error strong {
  color: var(--status-error);
}
</style>
