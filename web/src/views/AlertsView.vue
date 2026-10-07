<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQuery } from '@/composables/useQuery'
import { api, type RuleStatus } from '@/lib/api'
import { describeCondition, groupLabel } from '@/lib/alerts'
import AlertStatus from '@/components/AlertStatus.vue'
import ChannelsPanel from '@/components/ChannelsPanel.vue'
import RuleForm from '@/components/RuleForm.vue'
import SidePanel from '@/components/SidePanel.vue'
import StatTile from '@/components/StatTile.vue'
import StatusMessage from '@/components/StatusMessage.vue'

const route = useRoute()
const router = useRouter()

const rules = useQuery(() => 'rules', () => api.alertRules())
const events = useQuery(() => 'events', () => api.alertEvents())
const channels = useQuery(() => 'channels', () => api.channels())
const metrics = useQuery(() => 'metrics', () => api.metrics({ from: 'now-1h', to: 'now' }))

// Alert states change on the server: keep the page live.
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  timer = setInterval(() => {
    void rules.refresh()
    void events.refresh()
  }, 15_000)
})
onBeforeUnmount(() => clearInterval(timer))

const editing = computed(() => {
  const id = route.query.rule
  if (id === 'new') return 'new'
  return rules.data.value?.find((r) => r.id === id)
})
const firing = computed(() => (rules.data.value ?? []).filter((r) => r.status === 'firing').length)
const pending = computed(() => (rules.data.value ?? []).filter((r) => r.status === 'pending').length)
const channelNames = computed(() => new Map((channels.data.value ?? []).map((c) => [c.id, c.name])))

function open(id: string) {
  void router.push({ query: { ...route.query, rule: id } })
}
function close() {
  const { rule: _, ...rest } = route.query
  void router.push({ query: rest })
}
async function saved() {
  close()
  await Promise.all([rules.refresh(), events.refresh()])
}
function worst(r: RuleStatus) {
  const s = r.states.find((x) => x.status === 'firing') ?? r.states.find((x) => x.status === 'pending') ?? r.states[0]
  return s
}
const when = (iso: string) => new Date(iso).toLocaleString()
</script>

<template>
  <div class="page" :class="{ loading: rules.loading.value }">
    <section class="tiles">
      <StatTile label="Firing" :value="String(firing)" :status="firing ? 'error' : undefined" />
      <StatTile label="Pending" :value="String(pending)" />
      <StatTile label="Rules" :value="String(rules.data.value?.length ?? 0)" />
    </section>

    <div class="layout">
      <section>
        <div class="toolbar">
          <h2>Rules</h2>
          <button type="button" class="btn primary new-rule" @click="open('new')">New rule</button>
        </div>
        <StatusMessage v-if="rules.error.value" kind="error" title="Could not load rules" :detail="rules.error.value.message" />
        <StatusMessage
          v-else-if="rules.data.value?.length === 0"
          kind="empty"
          title="No alert rules yet"
          detail="Create a rule to be notified when errors spike or latency degrades."
        />
        <table v-else-if="rules.data.value" class="table">
          <thead>
            <tr>
              <th>Status</th>
              <th>Rule</th>
              <th class="num">Value</th>
              <th>Notifies</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in rules.data.value" :key="r.id" class="clickable" @click="open(r.id)">
              <td>
                <AlertStatus :status="r.status" />
                <span v-if="!r.enabled" class="muted"> disabled</span>
              </td>
              <td class="rule">
                <strong>{{ r.name }}</strong>
                <span class="muted mono">{{ describeCondition(r) }}</span>
                <span v-if="r.firing > 1" class="muted">{{ r.firing }} groups firing</span>
              </td>
              <td class="num">
                <template v-if="worst(r)">
                  {{ Number(worst(r)!.value.toPrecision(4)) }}
                  <span v-if="r.group_by?.length" class="muted"> · {{ groupLabel(worst(r)!.labels) }}</span>
                </template>
                <span v-else class="muted">—</span>
              </td>
              <td class="muted">{{ r.channels.map((id) => channelNames.get(id) ?? '?').join(', ') || '—' }}</td>
            </tr>
          </tbody>
        </table>
      </section>

      <aside class="side">
        <section class="events card">
          <h3>Recent events</h3>
          <p v-if="!events.data.value?.length" class="muted">Nothing has fired yet.</p>
          <div v-for="e in events.data.value" :key="e.id" class="event">
            <AlertStatus :status="e.status" />
            <div>
              <div>{{ e.rule_name }} <span v-if="Object.keys(e.labels).length" class="muted">· {{ groupLabel(e.labels) }}</span></div>
              <div class="muted small">{{ when(e.at) }} · value {{ Number(e.value.toPrecision(4)) }}</div>
            </div>
          </div>
        </section>
        <ChannelsPanel :channels="channels.data.value ?? []" @changed="channels.refresh()" />
      </aside>
    </div>

    <SidePanel v-if="editing" :title="editing === 'new' ? 'New alert rule' : editing.name" @close="close">
      <RuleForm
        :key="editing === 'new' ? 'new' : editing.id"
        :rule="editing === 'new' ? undefined : editing"
        :channels="channels.data.value ?? []"
        :metrics="metrics.data.value ?? []"
        @saved="saved"
        @deleted="saved"
      />
    </SidePanel>
  </div>
</template>

<style scoped>
.tiles {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-3);
  margin-bottom: var(--space-4);
}
.layout {
  display: grid;
  grid-template-columns: 1fr 340px;
  gap: var(--space-4);
}
.toolbar {
  justify-content: space-between;
}
h2 {
  margin: 0;
  font-size: 14px;
}
h3 {
  margin: 0 0 var(--space-2);
  font-size: 12px;
}
.rule {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding-top: 6px !important;
  padding-bottom: 6px !important;
  white-space: normal !important;
}
.side {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
.event {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: var(--space-2);
  align-items: start;
  padding: 6px 0;
  border-bottom: 1px solid var(--border);
}
.small {
  font-size: 11px;
}
</style>
