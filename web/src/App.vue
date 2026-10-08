<script setup lang="ts">
import { useRoute } from 'vue-router'
import AppSidebar from './components/AppSidebar.vue'
import AppTopBar from './components/AppTopBar.vue'

const route = useRoute()
</script>

<template>
  <RouterView v-if="route.meta.public" />
  <div v-else class="shell">
    <AppSidebar />
    <div class="main">
      <AppTopBar />
      <main class="content">
        <RouterView />
      </main>
    </div>
  </div>
</template>

<style scoped>
.shell {
  display: flex;
  height: 100vh;
}
.main {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}
.content {
  flex: 1;
  overflow: auto;
}
</style>
