<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { SECTIONS } from '@/sections'
import { useTheme } from '@/composables/useTheme'
import { session, signOut } from '@/lib/session'
import AppIcon from './AppIcon.vue'

const route = useRoute()
const router = useRouter()
const { theme, toggle } = useTheme()

const menuOpen = ref(false)
const menuRoot = ref<HTMLElement>()
const initial = computed(() => (session.user?.name || session.user?.email || '?').charAt(0).toUpperCase())
async function doSignOut() {
  menuOpen.value = false
  await signOut()
  await router.push('/login')
}
function onDocumentClick(e: MouseEvent) {
  if (menuOpen.value && !menuRoot.value?.contains(e.target as Node)) menuOpen.value = false
}
onMounted(() => document.addEventListener('click', onDocumentClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick))

// Navigation keeps the global time range, never page-specific filters.
const timeQuery = computed(() => {
  const { from, to } = route.query
  return typeof from === 'string' && typeof to === 'string' ? { from, to } : {}
})

const groups = computed(() => {
  const out: (typeof SECTIONS)[number][][] = []
  for (const s of SECTIONS) (out[s.group] ??= []).push(s)
  return out
})
</script>

<template>
  <aside class="rail">
    <RouterLink to="/" class="logo" aria-label="obsrv home">
      <svg viewBox="0 0 64 64" width="26" height="26" aria-hidden="true">
        <g fill="none" stroke="var(--accent)" stroke-width="6">
          <circle cx="32" cy="32" r="25" />
          <circle cx="32" cy="32" r="12" />
        </g>
        <circle cx="32" cy="32" r="4" fill="var(--accent)" />
      </svg>
    </RouterLink>

    <nav aria-label="Main">
      <div v-for="(group, i) in groups" :key="i" class="group">
        <RouterLink
          v-for="section in group"
          :key="section.path"
          :to="{ path: section.path, query: timeQuery }"
          :aria-label="section.name"
          :data-label="section.name"
          class="item"
          :class="{ active: section.path === '/' ? route.path === '/' : route.path.startsWith(section.path) }"
        >
          <AppIcon :name="section.icon" :size="20" />
        </RouterLink>
      </div>
    </nav>

    <button
      type="button"
      class="item theme"
      :aria-label="theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'"
      :data-label="theme === 'dark' ? 'Light theme' : 'Dark theme'"
      @click="toggle"
    >
      <AppIcon :name="theme === 'dark' ? 'sun' : 'moon'" :size="20" />
    </button>

    <div v-if="session.user" ref="menuRoot" class="account">
      <button type="button" class="avatar" :aria-expanded="menuOpen" aria-label="Account" @click="menuOpen = !menuOpen">
        {{ initial }}
      </button>
      <div v-if="menuOpen" class="user-menu" role="menu">
        <div class="who">
          <strong>{{ session.user.name }}</strong>
          <span class="muted">{{ session.user.email }}</span>
        </div>
        <RouterLink to="/settings" class="entry" role="menuitem" @click="menuOpen = false">Settings</RouterLink>
        <button type="button" class="entry sign-out" role="menuitem" @click="doSignOut">Sign out</button>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.rail {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: var(--rail-width);
  padding: var(--space-3) 0;
  background: var(--bg-rail);
  border-right: 1px solid var(--border);
}
.logo {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  margin-bottom: var(--space-4);
}
nav {
  display: flex;
  flex-direction: column;
  flex: 1;
  gap: var(--space-5);
}
.group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.item {
  position: relative;
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border: 0;
  border-radius: var(--radius);
  background: none;
  color: var(--text-muted);
  cursor: pointer;
}
.item:hover {
  background: var(--bg-hover);
  color: var(--text);
}
.item.active {
  background: var(--accent-soft);
  color: var(--accent-text);
}
.account {
  position: relative;
  margin-top: var(--space-2);
}
.avatar {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border: 0;
  border-radius: 50%;
  background: var(--accent);
  color: #fff;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}
.user-menu {
  position: absolute;
  left: calc(100% + 12px);
  bottom: 0;
  z-index: 30;
  min-width: 220px;
  padding: var(--space-1);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-elevated);
  box-shadow: var(--shadow);
}
.who {
  display: flex;
  flex-direction: column;
  padding: 8px 10px;
  border-bottom: 1px solid var(--border);
  margin-bottom: var(--space-1);
  font-size: 13px;
}
.entry {
  display: block;
  width: 100%;
  padding: 8px 10px;
  border: 0;
  border-radius: var(--radius-sm);
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.entry:hover {
  background: var(--bg-hover);
}
/* Label tooltip on hover and keyboard focus. */
.item::after {
  content: attr(data-label);
  position: absolute;
  left: calc(100% + 10px);
  top: 50%;
  z-index: 30;
  padding: 5px 9px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--bg-elevated);
  box-shadow: var(--shadow);
  color: var(--text);
  font-size: 13px;
  white-space: nowrap;
  opacity: 0;
  pointer-events: none;
  transform: translate(-4px, -50%);
  transition: opacity 100ms, transform 100ms;
}
.item:hover::after,
.item:focus-visible::after {
  opacity: 1;
  transform: translate(0, -50%);
}
</style>
