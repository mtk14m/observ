<script setup lang="ts">
import type { Section } from '@/sections'
import AppIcon from '@/components/AppIcon.vue'

defineProps<{ section: Section }>()
</script>

<template>
  <section class="empty">
    <AppIcon :name="section.icon" class="glyph" />
    <h2>{{ section.name }}</h2>
    <p>{{ section.hint }}</p>
  </section>
</template>

<style scoped>
.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  height: 100%;
  color: var(--text-muted);
  text-align: center;
}
.glyph {
  width: 32px;
  height: 32px;
  color: var(--accent);
}
h2 {
  margin: var(--space-2) 0 0;
  font-size: 16px;
  color: var(--text);
}
p {
  margin: 0;
}
pre {
  margin-top: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-elevated);
  font-family: var(--font-mono);
  font-size: 12px;
}
</style>
