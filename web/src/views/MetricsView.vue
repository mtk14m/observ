<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTimeRange } from '@/composables/useTimeRange'
import { useQuery } from '@/composables/useQuery'
import { resolveNs } from '@/composables/useResolvedRange'
import { api } from '@/lib/api'
import { aggregations, seriesLabel, valueFormatter } from '@/lib/metrics'
import { absolute } from '@/lib/timeRange'
import TimeSeriesChart from '@/components/charts/TimeSeriesChart.vue'
import StatusMessage from '@/components/StatusMessage.vue'
import AppIcon from '@/components/AppIcon.vue'

const route = useRoute()
const router = useRouter()
const { range, setRange } = useTimeRange()
const zoom = (r: { from: number; to: number }) => setRange(absolute(r.from, r.to))
const filter = ref('')

const metrics = useQuery(() => ({ ...range.value }), (r) => api.metrics(r))

const param = (k: string) => (typeof route.query[k] === 'string' ? (route.query[k] as string) : '')
const current = computed(() => {
  const list = metrics.data.value ?? []
  return list.find((m) => m.name === param('metric')) ?? list[0]
})
const aggOptions = computed(() => (current.value ? aggregations(current.value) : []))
const agg = computed(() => {
  const a = param('agg')
  return aggOptions.value.some((o) => o.value === a) ? a : (aggOptions.value[0]?.value ?? '')
})
const groupBy = computed(() => param('by'))
const isCounter = computed(() => current.value?.type === 'Sum' && current.value.monotonic)

const visible = computed(() =>
  (metrics.data.value ?? []).filter((m) => m.name.toLowerCase().includes(filter.value.toLowerCase())),
)

function update(patch: Record<string, string | undefined>) {
  void router.push({ query: { ...route.query, ...patch } })
}

const result = useQuery(
  () =>
    current.value
      ? { range: { ...range.value }, metric: current.value.name, agg: agg.value, by: groupBy.value }
      : null,
  async ({ range, metric, agg, by }) => {
    const bounds = resolveNs(range)
    const series = await api.queryMetric(range, { metric, agg, groupBy: by ? [by] : [] })
    return { series, ...bounds }
  },
)

const chartSeries = computed(() =>
  (result.data.value?.series ?? []).slice(0, 8).map((s) => ({
    label: seriesLabel(s.labels, current.value?.name ?? ''),
    points: s.points,
  })),
)
const hidden = computed(() => Math.max(0, (result.data.value?.series.length ?? 0) - 8))
const format = computed(() => (current.value ? valueFormatter(current.value, agg.value, isCounter.value) : String))
</script>

<template>
  <div class="layout">
    <aside class="metric-list">
      <label class="search-field">
        <AppIcon name="search" :size="16" />
        <input v-model="filter" class="input" type="search" placeholder="Filter metrics" aria-label="Filter metrics" />
      </label>
      <button
        v-for="m in visible"
        :key="m.name"
        type="button"
        :class="{ active: current?.name === m.name }"
        @click="update({ metric: m.name, agg: undefined, by: undefined })"
      >
        <span class="metric-name">{{ m.name }}</span>
        <span class="metric-meta">{{ m.type }} · {{ m.series }} series</span>
      </button>
      <p v-if="metrics.data.value?.length === 0" class="muted empty">No metrics in this time range.</p>
    </aside>

    <section class="page main" :class="{ loading: result.loading.value }">
      <StatusMessage v-if="metrics.error.value" kind="error" title="Could not load metrics" :detail="metrics.error.value.message" />
      <template v-else-if="current">
        <div class="toolbar builder">
          <span class="metric-title mono">{{ current.name }}</span>
          <label class="check">
            Show
            <select class="select" aria-label="Aggregation" :value="agg" @change="update({ agg: ($event.target as HTMLSelectElement).value })">
              <option v-for="o in aggOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
            </select>
          </label>
          <label class="check">
            by
            <select
              class="select"
              aria-label="Group by"
              :value="groupBy"
              @change="update({ by: ($event.target as HTMLSelectElement).value || undefined })"
            >
              <option value="">everything</option>
              <option v-for="k in current.attribute_keys" :key="k" :value="k">{{ k }}</option>
            </select>
          </label>
        </div>

        <div class="card">
          <StatusMessage v-if="result.error.value" kind="error" title="Query failed" :detail="result.error.value.message" />
          <StatusMessage v-else-if="result.data.value && chartSeries.length === 0" kind="empty" title="No data points in this time range" />
          <TimeSeriesChart
            v-else-if="result.data.value"
            :series="chartSeries"
            :from="result.data.value.from"
            :to="result.data.value.to"
            :format="format"
            :height="280"
            @zoom="zoom"
          />
          <p v-if="hidden" class="muted more">{{ hidden }} more series not shown. Group by a less detailed attribute.</p>
        </div>
      </template>
    </section>
  </div>
</template>

<style scoped>
.layout {
  display: grid;
  grid-template-columns: 280px 1fr;
  height: 100%;
}
.metric-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: var(--space-3);
  border-right: 1px solid var(--border);
  overflow: auto;
}
.metric-list .search-field {
  flex: none;
  min-width: 0;
  margin-bottom: var(--space-2);
}
.metric-list button {
  display: flex;
  flex-direction: column;
  gap: 1px;
  padding: 6px var(--space-2);
  border: 0;
  border-radius: var(--radius);
  background: none;
  color: var(--text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.metric-list button:hover {
  background: var(--bg-hover);
}
.metric-list button.active {
  background: var(--accent-soft);
}
.metric-name {
  font-family: var(--font-mono);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.metric-meta {
  color: var(--text-muted);
  font-size: 11px;
}
.empty {
  padding: var(--space-2);
}
.main {
  min-width: 0;
}
.metric-title {
  font-weight: 600;
  margin-right: var(--space-2);
}
.more {
  margin: var(--space-2) 0 0;
  font-size: 12px;
}
</style>
