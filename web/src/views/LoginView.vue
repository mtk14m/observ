<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/lib/api'
import { session } from '@/lib/session'
import { safeNext } from '@/lib/redirect'
import AuthLayout from '@/components/AuthLayout.vue'

const route = useRoute()
const router = useRouter()
const email = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  error.value = ''
  busy.value = true
  try {
    session.user = await api.auth.login(email.value, password.value)
    await router.replace(safeNext(route.query.next))
  } catch (e) {
    error.value = e instanceof Error ? e.message.replace(/^auth: /, '') : String(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AuthLayout title="Sign in to obsrv">
    <form @submit.prevent="submit">
      <label>Email <input v-model="email" type="email" class="input" autocomplete="username" required autofocus /></label>
      <label>Password <input v-model="password" type="password" class="input" autocomplete="current-password" required /></label>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <button type="submit" class="btn primary" :disabled="busy">Sign in</button>
    </form>
  </AuthLayout>
</template>
