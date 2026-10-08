<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTimeRange } from '@/composables/useTimeRange'
import { useQuery } from '@/composables/useQuery'
import { api, type TraceFilters } from '@/lib/api'
import { formatDateTime, formatDuration, formatTime } from '@/lib/format'
import { filters, removeFilter } from '@/lib/searchQuery'
import StatusMessage from '@/components/StatusMessage.vue'
import FacetChip from '@/components/FacetChip.vue'
import AppIcon from '@/components/AppIcon.vue'
import ComparePanel from '@/components/ComparePanel.vue'
import Val from '@/components/Val.vue'

/** A query that always applies, hidden from the chips (e.g. a service hub). */
const props = defineProps<{ scope?: string }>()

const route = useRoute()
const router = useRouter()
const { range } = useTimeRange()

const q = computed(() => (typeof route.query.q === 'string' ? route.query.q : ''))
const draft = ref(q.value)
watch(q, (v) => (draft.value = v))
const active = computed(() => filters(q.value))
const search = () => setFilter('q', draft.value)

const filtersFromUrl = computed<TraceFilters>(() => {
  const q = route.query
  const min = Number(q.min_ms)
  return {
    q: [props.scope, typeof q.q === 'string' ? q.q : ''].filter(Boolean).join(' ') || undefined,
    service: typeof q.service === 'string' && q.service ? q.service : undefined,
    errors: q.errors === 'true' || undefined,
    minDurationMs: Number.isFinite(min) && min > 0 ? min : undefined,
  }
})

function setFilter(key: string, value: string | undefined) {
  void router.push({ query: { ...route.query, [key]: value || undefined } })
}

const loadServices = async () =>
  (await api.services(range.value)).map((s) => ({ value: s.name, count: s.requests }))
const tab = computed(() => (route.query.tab === 'compare' ? 'compare' : 'list'))
const traces = useQuery(
  () => (tab.value === 'list' ? { range: { ...range.value }, filters: filtersFromUrl.value } : null),
  ({ range, filters }) => api.traces(range, filters),
)
const longest = computed(() => Math.max(1, ...(traces.data.value ?? []).map((t) => t.duration_nano)))
</script>

<template>
  <div class="traces" :class="{ loading: traces.loading.value }">
    <form class="search-row" role="search" @submit.prevent="search">
      <label class="search-field">
        <AppIcon name="search" :size="16" />
        <input
          v-model="draft"
          data-search
          type="search"
          class="input mono"
          placeholder="Search spans: service:checkout status:error payment.issuer:acme-bank"
          aria-label="Search traces"
        />
      </label>
      <button type="submit" class="btn">Search</button>
    </form>
    <section class="filters">
      <span v-for="f in active" :key="f.raw" class="chip active-filter">
        <span>{{ f.key }} <span class="op">{{ f.negated ? 'is not' : 'is' }}</span> <strong>{{ f.value }}</strong></span>
        <button type="button" class="remove" :aria-label="`Remove ${f.raw}`" @click="setFilter('q', removeFilter(q, f.raw))">
          <AppIcon name="close" :size="14" />
        </button>
      </span>
      <span v-if="filtersFromUrl.service" class="chip active-filter">
        <span>service <span class="op">is</span> <strong>{{ filtersFromUrl.service }}</strong></span>
        <button type="button" class="remove" aria-label="Remove service filter" @click="setFilter('service', undefined)">
          <AppIcon name="close" :size="14" />
        </button>
      </span>
      <FacetChip v-else label="service" facet-key="service" :load="loadServices" @select="(v: string) => setFilter('service', v)" />
      <button
        type="button"
        class="chip errors-only"
        :class="{ on: filtersFromUrl.errors }"
        :aria-pressed="filtersFromUrl.errors === true"
        @click="setFilter('errors', filtersFromUrl.errors ? undefined : 'true')"
      >
        <span class="dot" aria-hidden="true" /> Errors only
      </button>
      <label class="chip duration">
        duration ≥
        <input
          type="number"
          min="0"
          :value="filtersFromUrl.minDurationMs ?? ''"
          placeholder="0"
          aria-label="Minimum duration in ms"
          @change="setFilter('min_ms', ($event.target as HTMLInputElement).value)"
        />
        ms
      </label>
    </section>

    <div class="tabs">
      <button type="button" class="tab" :class="{ active: tab === 'list' }" @click="setFilter('tab', undefined)">
        Traces <span v-if="tab === 'list' && traces.data.value" class="count">{{ traces.data.value.length }}</span>
      </button>
      <button type="button" class="tab" :class="{ active: tab === 'compare' }" @click="setFilter('tab', 'compare')">
        Compare <span class="count">errors</span>
      </button>
    </div>

    <ComparePanel v-if="tab === 'compare'" signal="spans" :range="range" :q="filtersFromUrl.q ?? ''" />
    <template v-else>

    <StatusMessage v-if="traces.error.value" kind="error" title="Could not load traces" :detail="traces.error.value.message" />
    <StatusMessage
      v-else-if="traces.data.value?.length === 0"
      kind="empty"
      title="No matching traces"
      detail="Try a wider time range or fewer filters."
    />
    <table v-else-if="traces.data.value" class="table">
      <thead>
        <tr>
          <th>Time</th>
          <th>Trace</th>
          <th>Duration</th>
          <th class="num">Spans</th>
          <th class="num">Errors</th>
          <th>Services</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="t in traces.data.value" :key="t.trace_id">
          <td class="mono muted" :title="formatDateTime(t.start)">{{ formatTime(t.start) }}</td>
          <td>
            <Val k="service.name" :v="t.root_service" class="svc" />
            <RouterLink :to="`/traces/${t.trace_id}`" class="trace-link muted">{{ t.root_name }}</RouterLink>
          </td>
          <td>
            <div class="duration">
              <span class="dur-text">{{ formatDuration(t.duration_nano) }}</span>
              <span class="dur-track"><span class="dur-bar" :style="{ width: `${(t.duration_nano / longest) * 100}%` }" /></span>
            </div>
          </td>
          <td class="num">{{ t.span_count }}</td>
          <td class="num">
            <span v-if="t.error_count" class="pill error">⚠ {{ t.error_count }}</span>
            <span v-else class="faint">0</span>
          </td>
          <td class="muted">{{ t.services.join(', ') }}</td>
        </tr>
      </tbody>
    </table>
    </template>
  </div>
</template>

<style scoped>
.traces {
  transition: opacity 120ms;
}
.traces.loading {
  opacity: 0.6;
}
.search-row {
  display: flex;
  gap: var(--space-2);
  padding: var(--space-4) var(--space-5) 0;
}
.filters {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-5) var(--space-3);
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
.errors-only .dot {
  width: 8px;
  height: 8px;
  border: 1.5px solid var(--text-faint);
  border-radius: 50%;
}
.errors-only.on {
  border-color: var(--status-error);
  color: var(--pill-error-text);
}
.errors-only.on .dot {
  border-color: var(--status-error);
  background: var(--status-error);
}
.duration input {
  width: 64px;
  border: 0;
  background: none;
  color: var(--text);
  font: inherit;
  text-align: right;
  outline: none;
}
.tabs {
  padding: 0 var(--space-5);
}
.table th:first-child,
.table td:first-child {
  padding-left: var(--space-5);
}
.trace-link:hover .svc {
  color: var(--accent-text);
}
.svc {
  font-weight: 500;
}
.duration {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}
.dur-text {
  width: 64px;
  font-variant-numeric: tabular-nums;
}
.dur-track {
  width: 120px;
  height: 6px;
  border-radius: 3px;
  background: var(--bg-hover);
}
.dur-bar {
  display: block;
  height: 100%;
  min-width: 2px;
  border-radius: 3px;
  background: var(--accent);
  opacity: 0.8;
}
</style>
