<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import TimeRangePicker from './TimeRangePicker.vue'

const route = useRoute()
const title = computed(() => (route.meta.title as string | undefined) ?? '')
</script>

<template>
  <header class="topbar">
    <h1>{{ title }}</h1>
    <TimeRangePicker />
  </header>
</template>

<style scoped>
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 48px;
  padding: 0 var(--space-4);
  border-bottom: 1px solid var(--border);
  background: var(--bg);
}
h1 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
</style>
