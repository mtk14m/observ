<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <div class="auth">
    <div class="panel">
      <svg class="logo" viewBox="0 0 64 64" width="40" height="40" aria-hidden="true">
        <g fill="none" stroke="var(--accent)" stroke-width="6">
          <circle cx="32" cy="32" r="25" />
          <circle cx="32" cy="32" r="12" />
        </g>
        <circle cx="32" cy="32" r="4" fill="var(--accent)" />
      </svg>
      <h1>{{ title }}</h1>
      <p v-if="subtitle" class="muted">{{ subtitle }}</p>
      <slot />
    </div>
  </div>
</template>

<style scoped>
.auth {
  display: grid;
  place-items: center;
  min-height: 100vh;
  padding: var(--space-4);
  background: var(--bg-rail);
}
.panel {
  width: min(400px, 100%);
  padding: var(--space-6);
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--bg);
  box-shadow: var(--shadow);
}
.logo {
  display: block;
  margin-bottom: var(--space-4);
}
h1 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}
p {
  margin: 6px 0 0;
}
:slotted(form) {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  margin-top: var(--space-5);
}
:slotted(label) {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
  font-weight: 500;
}
:slotted(.btn.primary) {
  justify-content: center;
  margin-top: var(--space-2);
}
:slotted(.error) {
  margin: 0;
  color: var(--status-error);
}
</style>
