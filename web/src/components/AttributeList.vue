<script setup lang="ts">
import { computed } from 'vue'

import Val from './Val.vue'

const props = defineProps<{ title: string; attributes: Record<string, string> }>()
const entries = computed(() => Object.entries(props.attributes).sort(([a], [b]) => a.localeCompare(b)))
</script>

<template>
  <section v-if="entries.length" class="attrs">
    <h3>{{ title }}</h3>
    <dl>
      <template v-for="[k, v] in entries" :key="k">
        <dt>{{ k }}</dt>
        <dd><Val :k="k" :v="v" /></dd>
      </template>
    </dl>
  </section>
</template>

<style scoped>
.attrs {
  margin-bottom: var(--space-4);
}
h3 {
  margin: 0 0 var(--space-2);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
}
dl {
  display: grid;
  grid-template-columns: minmax(120px, max-content) 1fr;
  margin: 0;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  font-family: var(--font-mono);
  font-size: 12px;
}
dt,
dd {
  margin: 0;
  padding: 5px 8px;
  border-bottom: 1px solid var(--border);
  overflow-wrap: anywhere;
}
dt {
  color: var(--text-muted);
}
dl > :nth-last-child(-n + 2) {
  border-bottom: 0;
}
</style>
