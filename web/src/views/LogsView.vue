<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTimeRange } from '@/composables/useTimeRange'
import { useQuery } from '@/composables/useQuery'
import { resolveNs } from '@/composables/useResolvedRange'
import { api, type LogRecord } from '@/lib/api'
import { formatCompact, formatDateTime, formatTime } from '@/lib/format'
import { addFilter, filters, removeFilter } from '@/lib/searchQuery'
import { absolute } from '@/lib/timeRange'
import StackedBars from '@/components/charts/StackedBars.vue'
import SidePanel from '@/components/SidePanel.vue'
import AttributeList from '@/components/AttributeList.vue'
import StatusMessage from '@/components/StatusMessage.vue'
import LevelPill from '@/components/LevelPill.vue'
import FacetChip from '@/components/FacetChip.vue'
import AppIcon from '@/components/AppIcon.vue'
import Val from '@/components/Val.vue'

const LIMIT = 200
// Facets offered as chips, and the query key each one adds.
const FACETS = [
  { key: 'level', label: 'level', term: 'level' },
  { key: 'service.name', label: 'service.name', term: 'service' },
  { key: 'deployment.environment.name', label: 'environment', term: 'deployment.environment.name' },
]

const route = useRoute()
const router = useRouter()
const { range, setRange } = useTimeRange()
const zoom = (r: { from: number; to: number }) => setRange(absolute(r.from, r.to))

const q = computed(() => (typeof route.query.q === 'string' ? route.query.q : ''))
const draft = ref(q.value)
watch(q, (v) => (draft.value = v))
const active = computed(() => filters(q.value))

function setQuery(next: string) {
  void router.push({ query: { ...route.query, q: next || undefined } })
}
const search = () => setQuery(draft.value)

const loadFacet = (key: string) => api.logFacet(range.value, q.value, key)
function pickFacet(f: (typeof FACETS)[number], value: string) {
  setQuery(addFilter(q.value, f.term, f.key === 'level' ? value.toLowerCase() : value))
}

const result = useQuery(
  () => ({ range: { ...range.value }, q: q.value }),
  async ({ range, q }) => {
    const bounds = resolveNs(range)
    const [logs, histogram] = await Promise.all([api.logs(range, q, LIMIT), api.logHistogram(range, q)])
    return { logs, histogram, ...bounds }
  },
)
const total = computed(() =>
  (result.data.value?.histogram ?? []).reduce((n, b) => n + Object.values(b.counts).reduce((a, c) => a + c, 0), 0),
)

const selected = ref<LogRecord | null>(null)
</script>

<template>
  <div class="logs" :class="{ loading: result.loading.value }">
    <section class="filters">
      <form class="search-row" role="search" @submit.prevent="search">
        <label class="search-field">
          <AppIcon name="search" :size="16" />
          <input
            v-model="draft"
            data-search
            type="search"
            class="input mono"
            placeholder='Search for attribute:value or text, e.g. service:api level:error "timeout"'
            aria-label="Search logs"
          />
        </label>
        <button type="submit" class="btn">Search</button>
      </form>

      <div class="chips">
        <span v-for="f in active" :key="f.raw" class="chip active-filter">
          <span>{{ f.key }} <span class="op">{{ f.negated ? 'is not' : 'is' }}</span> <strong>{{ f.value }}</strong></span>
          <button type="button" class="remove" :aria-label="`Remove ${f.raw}`" @click="setQuery(removeFilter(q, f.raw))">
            <AppIcon name="close" :size="14" />
          </button>
        </span>
        <FacetChip
          v-for="f in FACETS"
          :key="f.key"
          :label="f.label"
          :facet-key="f.key"
          :load="() => loadFacet(f.key)"
          @select="(v: string) => pickFacet(f, v)"
        />
        <button v-if="q" type="button" class="btn ghost clear" @click="setQuery('')">Clear</button>
      </div>
    </section>

    <StatusMessage v-if="result.error.value" kind="error" title="Search failed" :detail="result.error.value.message" />

    <template v-else-if="result.data.value">
      <section class="histogram">
        <StackedBars :buckets="result.data.value.histogram" :from="result.data.value.from" :to="result.data.value.to" :height="110" @zoom="zoom" />
      </section>

      <div class="tabs">
        <button type="button" class="tab active">All logs <span class="count">{{ formatCompact(total) }}</span></button>
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
            <td><Val k="level" :v="l.severity"><LevelPill :level="l.severity" /></Val></td>
            <td><Val k="service.name" :v="l.service" /></td>
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
        <LevelPill :level="selected.severity" />
        <RouterLink v-if="selected.trace_id" :to="`/traces/${selected.trace_id}`" class="btn">
          View trace <AppIcon name="external" :size="14" />
        </RouterLink>
      </div>
      <pre class="body">{{ selected.body }}</pre>
      <AttributeList title="Attributes" :attributes="selected.attributes" />
      <AttributeList title="Resource" :attributes="selected.resource_attributes" />
    </SidePanel>
  </div>
</template>

<style scoped>
.logs {
  transition: opacity 120ms;
}
.logs.loading {
  opacity: 0.6;
}
.filters {
  padding: var(--space-4) var(--space-5) var(--space-3);
  border-bottom: 1px solid var(--border);
}
.search-row {
  display: flex;
  gap: var(--space-2);
  margin-bottom: var(--space-3);
}
.chips {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
}
.active-filter {
  padding-right: 4px;
  cursor: default;
}
.remove {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  border: 0;
  border-radius: 4px;
  background: none;
  color: var(--text-muted);
  cursor: pointer;
}
.remove:hover {
  background: var(--bg-hover);
  color: var(--text);
}
.histogram {
  padding: var(--space-4) var(--space-5) var(--space-2);
  border-bottom: 1px solid var(--border);
}
.tabs {
  padding: 0 var(--space-5);
}
.table th:first-child,
.table td:first-child {
  padding-left: var(--space-5);
}
.more {
  margin: var(--space-3) 0;
  text-align: center;
}
.detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--space-3);
}
.body {
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
</style>
