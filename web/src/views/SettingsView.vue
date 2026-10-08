<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useQuery } from '@/composables/useQuery'
import { api, type NewUser } from '@/lib/api'
import { session } from '@/lib/session'

const isAdmin = computed(() => session.user?.role === 'admin')

// Password
const pw = reactive({ current: '', next: '' })
const pwStatus = ref<{ ok: boolean; text: string } | null>(null)
async function changePassword() {
  pwStatus.value = null
  try {
    await api.auth.changePassword(pw.current, pw.next)
    Object.assign(pw, { current: '', next: '' })
    pwStatus.value = { ok: true, text: 'Password changed. Your other sessions were signed out.' }
  } catch (e) {
    pwStatus.value = { ok: false, text: e instanceof Error ? e.message : String(e) }
  }
}

// Users (admins only)
const users = useQuery(() => (isAdmin.value ? 'users' : null), () => api.users())
const draft = reactive<NewUser>({ name: '', email: '', password: '', role: 'member' })
const userError = ref('')
async function addUser() {
  userError.value = ''
  try {
    await api.createUser({ ...draft })
    Object.assign(draft, { name: '', email: '', password: '', role: 'member' })
    await users.refresh()
  } catch (e) {
    userError.value = e instanceof Error ? e.message : String(e)
  }
}
async function removeUser(id: string) {
  userError.value = ''
  try {
    await api.deleteUser(id)
    await users.refresh()
  } catch (e) {
    userError.value = e instanceof Error ? e.message : String(e)
  }
}
</script>

<template>
  <div class="page settings">
    <section class="block">
      <h2 class="section-title">Your account</h2>
      <p class="muted">{{ session.user?.name }} · {{ session.user?.email }} · {{ session.user?.role }}</p>
      <form class="password" @submit.prevent="changePassword">
        <input v-model="pw.current" name="current" type="password" class="input" placeholder="Current password"
          autocomplete="current-password" aria-label="Current password" required />
        <input v-model="pw.next" name="new" type="password" class="input" placeholder="New password (10+ characters)"
          autocomplete="new-password" aria-label="New password" required />
        <button type="submit" class="btn">Change password</button>
        <p v-if="pwStatus" :class="pwStatus.ok ? 'ok' : 'error'" :role="pwStatus.ok ? 'status' : 'alert'">{{ pwStatus.text }}</p>
      </form>
    </section>

    <section v-if="isAdmin" class="block users">
      <h2 class="section-title">Users</h2>
      <table class="table">
        <thead>
          <tr><th>Name</th><th>Email</th><th>Role</th><th /></tr>
        </thead>
        <tbody>
          <tr v-for="u in users.data.value ?? []" :key="u.id">
            <td>{{ u.name }}</td>
            <td class="muted">{{ u.email }}</td>
            <td><span class="pill" :class="u.role === 'admin' ? 'info' : 'debug'">{{ u.role }}</span></td>
            <td class="num">
              <button v-if="u.id !== session.user?.id" type="button" class="btn ghost delete" :aria-label="`Remove ${u.email}`"
                @click="removeUser(u.id)">Remove</button>
              <span v-else class="faint">you</span>
            </td>
          </tr>
        </tbody>
      </table>
      <form class="add-user" @submit.prevent="addUser">
        <input v-model="draft.name" name="name" class="input" placeholder="Name" aria-label="Name" required />
        <input v-model="draft.email" name="email" type="email" class="input" placeholder="Email" aria-label="Email" required />
        <input v-model="draft.password" name="password" type="password" class="input" placeholder="Initial password"
          aria-label="Initial password" autocomplete="new-password" required />
        <select v-model="draft.role" name="role" class="select" aria-label="Role">
          <option value="member">Member</option>
          <option value="admin">Admin</option>
        </select>
        <button type="submit" class="btn primary">Add user</button>
      </form>
      <p v-if="userError" class="error" role="alert">{{ userError }}</p>
    </section>
  </div>
</template>

<style scoped>
.settings {
  max-width: 900px;
}
.block {
  margin-bottom: var(--space-6);
  padding-bottom: var(--space-6);
  border-bottom: 1px solid var(--border);
}
.password {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: var(--space-2);
  margin-top: var(--space-3);
}
.password p {
  grid-column: 1 / -1;
  margin: 0;
}
.add-user {
  display: grid;
  grid-template-columns: 1fr 1.3fr 1fr 120px auto;
  gap: var(--space-2);
  margin-top: var(--space-4);
}
.ok {
  color: var(--status-ok);
}
.error {
  margin: var(--space-2) 0 0;
  color: var(--status-error);
}
</style>
