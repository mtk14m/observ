<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/lib/api'
import { session } from '@/lib/session'
import AuthLayout from '@/components/AuthLayout.vue'

const router = useRouter()
const form = reactive({ name: '', email: '', password: '', confirm: '' })
const error = ref('')
const busy = ref(false)

async function submit() {
  error.value = ''
  if (form.password !== form.confirm) {
    error.value = 'The passwords do not match.'
    return
  }
  busy.value = true
  try {
    session.user = await api.auth.setup({ name: form.name, email: form.email, password: form.password })
    await router.replace('/')
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AuthLayout title="Welcome to obsrv" subtitle="Create the administrator account to get started.">
    <form @submit.prevent="submit">
      <label>Name <input v-model="form.name" name="name" class="input" autocomplete="name" required /></label>
      <label>Email <input v-model="form.email" name="email" type="email" class="input" autocomplete="username" required /></label>
      <label>
        Password <span class="muted">(at least 10 characters)</span>
        <input v-model="form.password" name="password" type="password" class="input" autocomplete="new-password" required />
      </label>
      <label>Confirm password <input v-model="form.confirm" name="confirm" type="password" class="input" autocomplete="new-password" required /></label>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <button type="submit" class="btn primary" :disabled="busy">Create account</button>
    </form>
  </AuthLayout>
</template>
