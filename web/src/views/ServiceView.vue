<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useTimeRange } from '@/composables/useTimeRange'
import { useQuery } from '@/composables/useQuery'
import { resolveNs } from '@/composables/useResolvedRange'
import { api } from '@/lib/api'
import { formatMs, formatPercent, formatRate } from '@/lib/format'
import { absolute } from '@/lib/timeRange'
import TimeSeriesChart from '@/components/charts/TimeSeriesChart.vue'
import StatTile from '@/components/StatTile.vue'
import AppIcon from '@/components/AppIcon.vue'
import StatusMessage from '@/components/StatusMessage.vue'

const route = useRoute()
const { range, query, setRange } = useTimeRange()
const zoom = (r: { from: number; to: number }) => setRange(absolute(r.from, r.to))
const name = computed(() => String(route.params.name))

const detail = useQuery(
  () => ({ range: { ...range.value }, name: name.value }),
  async ({ range, name }) => ({ ...(await api.service(range, name)), ...resolveNs(range) }),
)

const d = computed(() => detail.data.value)
const stepSeconds = computed(() => (d.value ? d.value.step / 1e9 : 1))
const rate = computed(() => [
  { label: 'requests', points: (d.value?.timeline ?? []).map((p) => ({ t: p.t, v: p.requests / stepSeconds.value })) },
])
const errorRate = computed(() => [
  {
    label: 'error rate',
    points: (d.value?.timeline ?? []).map((p) => ({ t: p.t, v: p.requests ? p.errors / p.requests : 0 })),
  },
])
const latency = computed(() =>
  (['p50', 'p95', 'p99'] as const).map((q) => ({
    label: q,
    points: (d.value?.timeline ?? []).map((p) => ({ t: p.t, v: p[`${q}_ms`] })),
  })),
)
const tracesLink = computed(() => ({ path: '/traces', query: { ...query.value, service: name.value } }))
const logsLink = computed(() => ({ path: '/logs', query: { ...query.value, q: `service:${name.value}` } }))
</script>

<template>
  <div class="page" :class="{ loading: detail.loading.value }">
    <nav class="actions">
      <RouterLink :to="tracesLink" class="btn">View traces <AppIcon name="external" :size="14" /></RouterLink>
      <RouterLink :to="logsLink" class="btn">View logs <AppIcon name="external" :size="14" /></RouterLink>
    </nav>

    <StatusMessage v-if="detail.error.value" kind="error" title="Could not load the service" :detail="detail.error.value.message" />

    <template v-else-if="d">
      <section class="tiles">
        <StatTile label="Requests" :value="formatRate(d.summary.rate_per_second)" />
        <StatTile label="Error rate" :value="formatPercent(d.summary.error_rate)" :status="d.summary.errors ? 'error' : undefined" />
        <StatTile label="Latency p95" :value="formatMs(d.summary.p95_ms)" />
      </section>

      <section class="charts">
        <div class="card">
          <h3>Requests</h3>
          <TimeSeriesChart :series="rate" :from="d.from" :to="d.to" :format="formatRate" :height="160" @zoom="zoom" />
        </div>
        <div class="card">
          <h3>Error rate</h3>
          <TimeSeriesChart :series="errorRate" :from="d.from" :to="d.to" :format="formatPercent" :height="160" @zoom="zoom" />
        </div>
        <div class="card">
          <h3>Latency</h3>
          <TimeSeriesChart :series="latency" :from="d.from" :to="d.to" :format="formatMs" :height="160" @zoom="zoom" />
        </div>
      </section>

      <section class="bottom">
        <div class="card operations">
          <h3>Operations</h3>
          <table class="table">
            <thead>
              <tr>
                <th>Name</th>
                <th class="num">Requests</th>
                <th class="num">Errors</th>
                <th class="num">p50</th>
                <th class="num">p95</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="o in d.operations" :key="o.name">
                <td class="mono">{{ o.name }}</td>
                <td class="num">{{ o.requests }}</td>
                <td class="num">
                  <span v-if="o.errors" class="err">{{ o.errors }}</span><span v-else class="muted">0</span>
                </td>
                <td class="num">{{ formatMs(o.p50_ms) }}</td>
                <td class="num">{{ formatMs(o.p95_ms) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="card deps">
          <h3>Called by</h3>
          <p v-if="!d.called_by.length" class="muted">No callers: this is an entry point.</p>
          <RouterLink v-for="c in d.called_by" :key="c.service" :to="{ path: `/services/${c.service}`, query }" class="dep">
            <span>{{ c.service }}</span><span class="muted">{{ c.requests }} calls</span>
          </RouterLink>
          <h3>Calls</h3>
          <p v-if="!d.calls.length" class="muted">No downstream services.</p>
          <RouterLink v-for="c in d.calls" :key="c.service" :to="{ path: `/services/${c.service}`, query }" class="dep">
            <span>{{ c.service }}</span>
            <span class="muted">{{ c.requests }} calls<template v-if="c.errors"> · <span class="err">{{ c.errors }} failed</span></template></span>
          </RouterLink>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  margin-bottom: var(--space-3);
}
.tiles {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: var(--space-3);
  margin-bottom: var(--space-3);
}
.charts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: var(--space-3);
}
.charts .card {
  margin: 0;
}
h3 {
  margin: 0 0 var(--space-2);
  font-size: 13px;
  font-weight: 600;
}
.bottom {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: var(--space-3);
  margin-top: var(--space-3);
}
.deps h3:not(:first-child) {
  margin-top: var(--space-4);
}
.dep {
  display: flex;
  justify-content: space-between;
  padding: 6px var(--space-2);
  border-radius: var(--radius);
}
.dep:hover {
  background: var(--bg-hover);
}
.err {
  color: var(--status-error);
  font-weight: 600;
}
</style>
