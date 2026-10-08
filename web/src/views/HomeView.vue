<script setup lang="ts">
import { computed } from 'vue'
import { useTimeRange } from '@/composables/useTimeRange'
import { useQuery } from '@/composables/useQuery'
import { api } from '@/lib/api'
import { serviceHealth } from '@/lib/health'
import { shiftRange } from '@/lib/palette'
import { formatDateTime, formatMs, formatPercent, formatRate } from '@/lib/format'
import StatusMessage from '@/components/StatusMessage.vue'

const { range, query } = useTimeRange()

const data = useQuery(
  () => ({ ...range.value }),
  async (r) => {
    const previous = shiftRange(r, -1, new Date())
    const [now, before, rules, issues, deployments] = await Promise.all([
      api.services(r),
      previous ? api.services(previous) : Promise.resolve([]),
      api.alertRules(),
      api.issues(r),
      api.deployments(r),
    ])
    return { health: serviceHealth(now, before), rules, issues, deployments }
  },
)

const d = computed(() => data.data.value)
const unhealthy = computed(() => (d.value?.health ?? []).filter((s) => s.status !== 'ok').length)
const firing = computed(() => (d.value?.rules ?? []).filter((r) => r.status === 'firing'))
const fresh = computed(() => (d.value?.issues ?? []).filter((i) => i.status !== 'ongoing').slice(0, 6))
const calm = computed(() => !firing.value.length && !fresh.value.length && !d.value?.deployments.length)
const headline = computed(() => {
  const n = unhealthy.value
  if (n === 0) return 'All services look healthy'
  return `${n} service${n > 1 ? 's need' : ' needs'} attention`
})
const STATUS_LABEL = { critical: 'Critical', warning: 'Degraded', ok: 'Healthy' }
</script>

<template>
  <div class="page home" :class="{ loading: data.loading.value }">
    <StatusMessage v-if="data.error.value" kind="error" title="Could not load the overview" :detail="data.error.value.message" />

    <template v-else-if="d">
      <header class="summary">
        <h2 :class="{ bad: unhealthy > 0 }">{{ headline }}</h2>
        <div class="counts">
          <RouterLink to="/alerts" class="count-chip" :class="{ hot: firing.length }">
            {{ firing.length }} alert{{ firing.length === 1 ? '' : 's' }} firing
          </RouterLink>
          <RouterLink :to="{ path: '/issues', query }" class="count-chip" :class="{ hot: fresh.length }">
            {{ fresh.length }} new or rising issue{{ fresh.length === 1 ? '' : 's' }}
          </RouterLink>
          <span class="count-chip">{{ d.deployments.length }} deployment{{ d.deployments.length === 1 ? '' : 's' }}</span>
        </div>
      </header>

      <div class="grid">
        <section class="health">
          <h3 class="section-title">Services</h3>
          <StatusMessage v-if="!d.health.length" kind="empty" title="No services yet"
            detail="Services appear as soon as obsrv receives spans." />
          <table v-else class="table">
            <thead>
              <tr>
                <th>Service</th>
                <th>Why</th>
                <th class="num">Rate</th>
                <th class="num">Errors</th>
                <th class="num">p95</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="s in d.health" :key="s.name">
                <td>
                  <RouterLink :to="{ path: `/services/${s.name}`, query }" class="svc">
                    <span class="light" :class="s.status" :aria-label="STATUS_LABEL[s.status]" />
                    {{ s.name }}
                  </RouterLink>
                </td>
                <td :class="s.reasons.length ? 'reason' : 'faint'">{{ s.reasons.join(' · ') || 'Healthy' }}</td>
                <td class="num">{{ formatRate(s.rate_per_second) }}</td>
                <td class="num">{{ formatPercent(s.error_rate) }}</td>
                <td class="num">
                  {{ formatMs(s.p95_ms) }}
                  <span v-if="s.p95Ratio && s.p95Ratio >= 1.5" class="up" aria-label="slower than before">↑</span>
                </td>
              </tr>
            </tbody>
          </table>
        </section>

        <aside class="attention">
          <h3 class="section-title">Needs attention</h3>
          <p v-if="calm" class="muted">Nothing needs attention right now.</p>
          <RouterLink v-for="r in firing" :key="r.id" :to="{ path: '/alerts', query: { rule: r.id } }" class="item">
            <span class="pill error">⚠ Firing</span>
            <span>{{ r.name }}</span>
          </RouterLink>
          <RouterLink v-for="i in fresh" :key="i.id" :to="{ path: '/issues', query: { ...query, issue: i.id } }" class="item">
            <span class="pill" :class="i.status === 'new' ? 'error' : 'warn'">{{ i.status === 'new' ? 'New' : 'Rising' }}</span>
            <span>
              <span class="mono">{{ i.title }}</span>
              <span class="muted small">{{ i.service }} · {{ i.count }} in this period</span>
            </span>
          </RouterLink>
          <div v-for="dep in d.deployments" :key="dep.service + dep.version" class="item">
            <span class="pill info">Deploy</span>
            <span>
              <span>{{ dep.service }} {{ dep.previous }} → {{ dep.version }}</span>
              <span class="muted small">{{ formatDateTime(dep.at) }}</span>
            </span>
          </div>
        </aside>
      </div>
    </template>
  </div>
</template>

<style scoped>
.summary {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  margin-bottom: var(--space-5);
}
h2 {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
}
h2.bad {
  color: var(--text);
}
.counts {
  display: flex;
  gap: var(--space-2);
}
.count-chip {
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: 999px;
  color: var(--text-muted);
  font-size: 13px;
}
.count-chip.hot {
  border-color: var(--status-error);
  color: var(--pill-error-text);
}
.grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 380px;
  gap: var(--space-6);
}
.svc {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-weight: 500;
}
.svc:hover {
  color: var(--accent-text);
}
.light {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--status-ok);
}
.light.warning {
  background: var(--status-warn);
}
.light.critical {
  background: var(--status-error);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--status-error) 25%, transparent);
}
.reason {
  color: var(--text);
}
.up {
  color: var(--status-error);
  font-weight: 600;
}
.attention {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.item {
  display: grid;
  grid-template-columns: 70px 1fr;
  align-items: start;
  gap: var(--space-3);
  padding: 10px var(--space-2);
  border-radius: var(--radius);
}
a.item:hover {
  background: var(--bg-hover);
}
.item .pill {
  justify-content: center;
  font-size: 12px;
}
.item > span:last-child {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.small {
  font-size: 12px;
}
</style>
