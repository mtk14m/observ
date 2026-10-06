<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTimeRange } from '@/composables/useTimeRange'
import { useQuery } from '@/composables/useQuery'
import { api, type TraceFilters } from '@/lib/api'
import { formatDateTime, formatDuration } from '@/lib/format'
import StatusMessage from '@/components/StatusMessage.vue'

const route = useRoute()
const router = useRouter()
const { range } = useTimeRange()

const filters = computed<TraceFilters>(() => {
  const q = route.query
  const min = Number(q.min_ms)
  return {
    service: typeof q.service === 'string' && q.service ? q.service : undefined,
    errors: q.errors === 'true' || undefined,
    minDurationMs: Number.isFinite(min) && min > 0 ? min : undefined,
  }
})

function setFilter(key: string, value: string | undefined) {
  void router.push({ query: { ...route.query, [key]: value || undefined } })
}

const services = useQuery(() => ({ ...range.value }), (r) => api.services(r))
const traces = useQuery(
  () => ({ range: { ...range.value }, filters: filters.value }),
  ({ range, filters }) => api.traces(range, filters),
)
const longest = computed(() => Math.max(1, ...(traces.data.value ?? []).map((t) => t.duration_nano)))
</script>

<template>
  <div class="page" :class="{ loading: traces.loading.value }">
    <div class="toolbar">
      <select
        class="select"
        aria-label="Service"
        :value="filters.service ?? ''"
        @change="setFilter('service', ($event.target as HTMLSelectElement).value)"
      >
        <option value="">All services</option>
        <option v-for="s in services.data.value ?? []" :key="s.name" :value="s.name">{{ s.name }}</option>
      </select>
      <label class="check">
        <input
          type="checkbox"
          :checked="filters.errors === true"
          @change="setFilter('errors', ($event.target as HTMLInputElement).checked ? 'true' : undefined)"
        />
        Errors only
      </label>
      <label class="check">
        Min duration
        <input
          type="number"
          min="0"
          class="input num-input"
          :value="filters.minDurationMs ?? ''"
          placeholder="ms"
          @change="setFilter('min_ms', ($event.target as HTMLInputElement).value)"
        />
        ms
      </label>
    </div>

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
          <td class="mono muted">{{ formatDateTime(t.start) }}</td>
          <td>
            <RouterLink :to="`/traces/${t.trace_id}`" class="trace-link">
              <span class="svc">{{ t.root_service }}</span> {{ t.root_name }}
            </RouterLink>
          </td>
          <td class="duration">
            <span class="dur-bar" :style="{ width: `${(t.duration_nano / longest) * 100}%` }" aria-hidden="true" />
            <span class="dur-text">{{ formatDuration(t.duration_nano) }}</span>
          </td>
          <td class="num">{{ t.span_count }}</td>
          <td class="num">
            <span v-if="t.error_count" class="err">⚠ {{ t.error_count }}</span>
            <span v-else class="muted">0</span>
          </td>
          <td class="muted">{{ t.services.join(', ') }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.num-input {
  width: 80px;
}
.trace-link:hover {
  color: var(--accent);
}
.svc {
  font-weight: 600;
}
.duration {
  position: relative;
  min-width: 140px;
}
.dur-bar {
  position: absolute;
  left: var(--space-3);
  top: 50%;
  height: 4px;
  max-width: calc(100% - 2 * var(--space-3));
  transform: translateY(-50%);
  border-radius: 2px;
  background: var(--accent-soft);
}
.dur-text {
  position: relative;
  font-variant-numeric: tabular-nums;
}
.err {
  color: var(--status-error);
  font-weight: 600;
}
</style>
