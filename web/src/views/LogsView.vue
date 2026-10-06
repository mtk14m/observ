<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTimeRange } from '@/composables/useTimeRange'
import { useQuery } from '@/composables/useQuery'
import { resolveNs } from '@/composables/useResolvedRange'
import { api, type LogRecord } from '@/lib/api'
import { formatDateTime, formatTime } from '@/lib/format'
import StackedBars from '@/components/charts/StackedBars.vue'
import SidePanel from '@/components/SidePanel.vue'
import AttributeList from '@/components/AttributeList.vue'
import StatusMessage from '@/components/StatusMessage.vue'
import SeverityLabel from '@/components/SeverityLabel.vue'

const LIMIT = 200
const route = useRoute()
const router = useRouter()
const { range } = useTimeRange()

const q = computed(() => (typeof route.query.q === 'string' ? route.query.q : ''))
const draft = ref(q.value)
watch(q, (v) => (draft.value = v))

function search() {
  void router.push({ query: { ...route.query, q: draft.value || undefined } })
}

const result = useQuery(
  () => ({ range: { ...range.value }, q: q.value }),
  async ({ range, q }) => {
    const bounds = resolveNs(range)
    const [logs, histogram] = await Promise.all([api.logs(range, q, LIMIT), api.logHistogram(range, q)])
    return { logs, histogram, ...bounds }
  },
)

const selected = ref<LogRecord | null>(null)
</script>

<template>
  <div class="page" :class="{ loading: result.loading.value }">
    <form class="toolbar" role="search" @submit.prevent="search">
      <input
        v-model="draft"
        type="search"
        class="input mono search"
        placeholder='Search logs: service:api level:error "timeout" -health'
        aria-label="Search logs"
      />
      <button type="submit" class="btn">Search</button>
    </form>

    <StatusMessage v-if="result.error.value" kind="error" title="Search failed" :detail="result.error.value.message" />

    <template v-else-if="result.data.value">
      <div class="card">
        <StackedBars :buckets="result.data.value.histogram" :from="result.data.value.from" :to="result.data.value.to" />
      </div>

      <StatusMessage
        v-if="result.data.value.logs.length === 0"
        kind="empty"
        title="No matching logs"
        detail="Try a wider time range or a simpler search."
      />
      <table v-else class="table">
        <thead>
          <tr>
            <th>Time</th>
            <th>Level</th>
            <th>Service</th>
            <th>Message</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(l, i) in result.data.value.logs"
            :key="i"
            class="clickable"
            :class="{ selected: selected === l }"
            @click="selected = l"
          >
            <td class="mono muted">{{ formatTime(l.time) }}</td>
            <td><SeverityLabel :level="l.severity" /></td>
            <td>{{ l.service }}</td>
            <td class="mono truncate">{{ l.body }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="result.data.value.logs.length === LIMIT" class="muted more">
        Showing the {{ LIMIT }} most recent records. Narrow the search or the time range to see older ones.
      </p>
    </template>

    <SidePanel v-if="selected" :title="`${selected.service} · ${formatDateTime(selected.time)}`" @close="selected = null">
      <div class="detail-head">
        <SeverityLabel :level="selected.severity" />
        <RouterLink v-if="selected.trace_id" :to="`/traces/${selected.trace_id}`" class="link">View trace →</RouterLink>
      </div>
      <pre class="body">{{ selected.body }}</pre>
      <AttributeList title="Attributes" :attributes="selected.attributes" />
      <AttributeList title="Resource" :attributes="selected.resource_attributes" />
    </SidePanel>
  </div>
</template>

<style scoped>
.search {
  flex: 1;
  min-width: 240px;
}
.more {
  margin: var(--space-3) 0 0;
  text-align: center;
}
.detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-3);
}
.link {
  color: var(--accent);
}
.body {
  margin: 0 0 var(--space-4);
  padding: var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg);
  font-family: var(--font-mono);
  font-size: 12px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
