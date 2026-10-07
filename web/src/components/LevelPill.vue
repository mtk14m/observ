<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ level: string }>()
const kind = computed(() => {
  const l = props.level.toUpperCase()
  if (l === 'FATAL' || l === 'ERROR') return 'error'
  if (l === 'WARN' || l === 'WARNING') return 'warn'
  if (l === 'INFO') return 'info'
  return 'debug'
})
const label = computed(() => {
  const l = props.level || 'unset'
  return l.charAt(0).toUpperCase() + l.slice(1).toLowerCase()
})
</script>

<template>
  <span class="pill" :class="kind">{{ label }}</span>
</template>
