<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTimeRange } from '@/composables/useTimeRange'
import { useQuery } from '@/composables/useQuery'
import { api } from '@/lib/api'
import { issueLinks } from '@/lib/health'
import { formatCompact, formatDateTime, formatTime } from '@/lib/format'
import Sparkline from '@/components/Sparkline.vue'
import SidePanel from '@/components/SidePanel.vue'
import StatusMessage from '@/components/StatusMessage.vue'
import Val from '@/components/Val.vue'
import AppIcon from '@/components/AppIcon.vue'

/** Only list the issues of this service (service hub). */
const props = defineProps<{ service?: string }>()

const route = useRoute()
const router = useRouter()
const { range, query } = useTimeRange()
const issues = useQuery(
  () => ({ range: { ...range.value }, service: props.service }),
  async ({ range, service }) => (await api.issues(range)).filter((i) => !service || i.service === service),
)

const selected = computed(() => issues.data.value?.find((i) => i.id === route.query.issue))
const links = computed(() => (selected.value ? issueLinks(selected.value) : null))
const withTime = (to: { path: string; query?: Record<string, string> }) => ({ ...to, query: { ...query.value, ...to.query } })

function open(id: string) {
  void router.push({ query: { ...route.query, issue: id } })
}
function close() {
  const { issue: _, ...rest } = route.query
  void router.push({ query: rest })
}
const STATUS = { new: { label: 'NEW', cls: 'error' }, rising: { label: 'RISING', cls: 'warn' }, ongoing: { label: 'ongoing', cls: 'debug' } }
</script>

<template>
  <div class="issues" :class="{ loading: issues.loading.value }">
    <div class="tabs">
      <span class="tab active">Issues <span class="count">{{ issues.data.value?.length ?? 0 }}</span></span>
    </div>
    <StatusMessage v-if="issues.error.value" kind="error" title="Could not load issues" :detail="issues.error.value.message" />
    <StatusMessage
      v-else-if="issues.data.value?.length === 0"
      kind="empty"
      title="No errors in this period"
      detail="Error logs and spans in error are grouped here into issues."
    />
    <table v-else-if="issues.data.value" class="table">
      <thead>
        <tr>
          <th>Status</th>
          <th>Issue</th>
          <th>Trend</th>
          <th class="num">Events</th>
          <th>Last seen</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="i in issues.data.value"
          :key="i.id"
          class="clickable"
          :class="{ selected: selected?.id === i.id }"
          @click="open(i.id)"
        >
          <td><span class="pill" :class="STATUS[i.status].cls">{{ STATUS[i.status].label }}</span></td>
          <td class="issue-cell">
            <span class="mono title">{{ i.title }}</span>
            <span class="meta">
              <Val k="service.name" :v="i.service" /> · <span class="muted">{{ i.kind === 'log' ? 'error logs' : 'spans in error' }}</span>
            </span>
          </td>
          <td><Sparkline :values="i.buckets" /></td>
          <td class="num">
            {{ formatCompact(i.count) }}
            <span v-if="i.previous_count" class="faint small">was {{ formatCompact(i.previous_count) }}</span>
          </td>
          <td class="muted mono">{{ formatTime(i.last_seen) }}</td>
        </tr>
      </tbody>
    </table>

    <SidePanel v-if="selected && links" :title="selected.title" @close="close">
      <dl class="facts">
        <dt>Service</dt>
        <dd><Val k="service.name" :v="selected.service" /></dd>
        <dt>Events</dt>
        <dd>{{ selected.count }} in this period · {{ selected.previous_count }} in the previous one</dd>
        <dt>First seen</dt>
        <dd>{{ formatDateTime(selected.first_seen) }}</dd>
        <dt>Last seen</dt>
        <dd>{{ formatDateTime(selected.last_seen) }}</dd>
      </dl>
      <h3 class="section-title">Latest example</h3>
      <pre class="example">{{ selected.example }}</pre>
      <div class="actions">
        <RouterLink v-if="links.trace" :to="links.trace" class="btn primary">Open example trace</RouterLink>
        <RouterLink :to="withTime(links.traces as never)" class="btn">Traces in error <AppIcon name="external" :size="14" /></RouterLink>
        <RouterLink v-if="selected.kind === 'log'" :to="withTime(links.logs as never)" class="btn">Matching logs <AppIcon name="external" :size="14" /></RouterLink>
        <RouterLink :to="{ path: '/traces', query: { ...query, q: `service:${selected.service}`, tab: 'compare' } }" class="btn">
          What do errors have in common?
        </RouterLink>
      </div>
    </SidePanel>
  </div>
</template>

<style scoped>
.issues {
  transition: opacity 120ms;
}
.issues.loading {
  opacity: 0.6;
}
.tabs {
  padding: var(--space-3) var(--space-5) 0;
}
.table th:first-child,
.table td:first-child {
  padding-left: var(--space-5);
}
.issue-cell {
  white-space: normal !important;
  padding-top: 8px !important;
  padding-bottom: 8px !important;
}
.title {
  display: block;
}
.meta {
  font-size: 13px;
}
.small {
  display: block;
  font-size: 11px;
}
.facts {
  display: grid;
  grid-template-columns: 100px 1fr;
  gap: 6px var(--space-3);
  margin: 0 0 var(--space-4);
}
.facts dt {
  color: var(--text-muted);
}
.facts dd {
  margin: 0;
}
.example {
  margin: 0 0 var(--space-4);
  padding: var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-subtle);
  font-family: var(--font-mono);
  font-size: 13px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
</style>
