<script setup lang="ts">
import { reactive, ref } from 'vue'
import { api, type Channel } from '@/lib/api'

defineProps<{ channels: Channel[] }>()
const emit = defineEmits<{ changed: [] }>()

const draft = reactive<Channel>({ name: '', type: 'slack', url: '' })
const error = ref('')
const results = reactive<Record<string, string>>({})

async function add() {
  error.value = ''
  try {
    await api.createChannel({ ...draft })
    Object.assign(draft, { name: '', url: '' })
    emit('changed')
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  }
}

async function test(c: Channel) {
  results[c.id!] = 'Sending…'
  try {
    await api.testChannel(c.id!)
    results[c.id!] = 'Sent ✓'
  } catch (e) {
    results[c.id!] = `Failed: ${e instanceof Error ? e.message : e}`
  }
}

async function remove(c: Channel) {
  try {
    await api.deleteChannel(c.id!)
    emit('changed')
  } catch (e) {
    results[c.id!] = e instanceof Error ? e.message : String(e)
  }
}
</script>

<template>
  <section class="channels card">
    <h3>Channels</h3>
    <ul>
      <li v-for="c in channels" :key="c.id">
        <div>
          <strong>{{ c.name }}</strong> <span class="muted">{{ c.type }}</span>
          <div v-if="results[c.id!]" class="result">{{ results[c.id!] }}</div>
        </div>
        <div class="buttons">
          <button type="button" class="btn test" @click="test(c)">Test</button>
          <button type="button" class="btn danger" :aria-label="`Delete ${c.name}`" @click="remove(c)">✕</button>
        </div>
      </li>
    </ul>
    <form class="add" @submit.prevent="add">
      <input v-model="draft.name" name="channel-name" class="input" placeholder="Name" aria-label="Channel name" />
      <select v-model="draft.type" name="channel-type" class="select" aria-label="Channel type">
        <option value="slack">Slack</option>
        <option value="webhook">Webhook</option>
      </select>
      <input v-model="draft.url" name="channel-url" class="input mono" placeholder="https://hooks.slack.com/services/…" aria-label="URL" />
      <button type="submit" class="btn">Add channel</button>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
    </form>
  </section>
</template>

<style scoped>
h3 {
  margin: 0 0 var(--space-2);
  font-size: 12px;
}
ul {
  margin: 0 0 var(--space-3);
  padding: 0;
  list-style: none;
}
li {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 0;
  border-bottom: 1px solid var(--border);
}
.buttons {
  display: flex;
  gap: 4px;
}
.result {
  font-size: 11px;
  color: var(--text-muted);
}
.add {
  display: grid;
  grid-template-columns: 1fr 110px;
  gap: var(--space-2);
}
.add .mono,
.add button,
.add .error {
  grid-column: 1 / -1;
}
.error {
  margin: 0;
  color: var(--status-error);
}
</style>
