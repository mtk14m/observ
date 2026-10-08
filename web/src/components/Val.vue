<script setup lang="ts">
import { openValueMenu } from '@/composables/useValueMenu'

/** A value the user can act on: filter, exclude, open logs or traces… */
const props = defineProps<{ k: string; v: string }>()

function open(e: MouseEvent) {
  e.stopPropagation() // do not also select the row the value sits in
  openValueMenu(e, props.k, props.v)
}
</script>

<template>
  <button type="button" class="val" :title="`${k}: ${v}`" @click="open"><slot>{{ v }}</slot></button>
</template>

<style scoped>
.val {
  max-width: 100%;
  padding: 0 3px;
  margin: 0 -3px;
  border: 0;
  border-radius: 4px;
  background: none;
  color: inherit;
  font: inherit;
  text-align: inherit;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
}
.val:hover {
  background: var(--accent-soft);
  color: var(--accent-text);
}
</style>
