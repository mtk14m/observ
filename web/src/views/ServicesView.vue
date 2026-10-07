<script setup lang="ts">
import { useTimeRange } from '@/composables/useTimeRange'
import { useQuery } from '@/composables/useQuery'
import { api } from '@/lib/api'
import { formatCompact, formatMs, formatPercent, formatRate } from '@/lib/format'
import StatusMessage from '@/components/StatusMessage.vue'
import ServiceMap from '@/components/ServiceMap.vue'

const { range, query } = useTimeRange()
const services = useQuery(() => ({ ...range.value }), (r) => api.services(r))
const edges = useQuery(() => ({ ...range.value }), (r) => api.serviceMap(r))
</script>

<template>
  <div class="page" :class="{ loading: services.loading.value }">
    <StatusMessage v-if="services.error.value" kind="error" title="Could not load services" :detail="services.error.value.message" />

    <StatusMessage
      v-else-if="services.data.value?.length === 0"
      kind="empty"
      title="No services yet"
      detail="Services appear as soon as obsrv receives spans. Point any OpenTelemetry SDK or Collector at obsrv:"
    >
      <pre class="hint"><code>OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318</code></pre>
    </StatusMessage>

    <template v-else-if="services.data.value">
    <h2 v-if="edges.data.value?.length" class="section-title">Service map</h2>
    <ServiceMap
      v-if="edges.data.value?.length"
      :edges="edges.data.value"
      :services="services.data.value"
      :query="query"
    />
    <div class="tabs">
      <span class="tab active">All services <span class="count">{{ services.data.value.length }}</span></span>
    </div>
    <table class="table">
      <thead>
        <tr>
          <th>Service</th>
          <th class="num">Requests</th>
          <th class="num">Rate</th>
          <th class="num">Error rate</th>
          <th class="num">p50</th>
          <th class="num">p95</th>
          <th class="num">p99</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="s in services.data.value" :key="s.name">
          <td>
            <RouterLink :to="{ path: `/services/${s.name}`, query }" class="name">{{ s.name }}</RouterLink>
          </td>
          <td class="num">{{ formatCompact(s.requests) }}</td>
          <td class="num">{{ formatRate(s.rate_per_second) }}</td>
          <td class="num">
            <span v-if="s.errors > 0" class="err-dot" aria-hidden="true" />{{ formatPercent(s.error_rate) }}
          </td>
          <td class="num">{{ formatMs(s.p50_ms) }}</td>
          <td class="num">{{ formatMs(s.p95_ms) }}</td>
          <td class="num">{{ formatMs(s.p99_ms) }}</td>
        </tr>
      </tbody>
    </table>
    </template>
  </div>
</template>

<style scoped>
.name {
  font-weight: 500;
}
.name:hover {
  color: var(--accent);
}
.err-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  margin-right: 6px;
  border-radius: 50%;
  background: var(--status-error);
  vertical-align: 1px;
}
.hint {
  display: inline-block;
  margin-top: var(--space-3);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--bg-elevated);
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--text);
}
</style>
