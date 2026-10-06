<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useQuery } from '@/composables/useQuery'
import { api, type Span } from '@/lib/api'
import { buildWaterfall } from '@/lib/trace'
import { formatDateTime, formatDuration, formatTime } from '@/lib/format'
import SidePanel from '@/components/SidePanel.vue'
import AttributeList from '@/components/AttributeList.vue'
import StatusMessage from '@/components/StatusMessage.vue'
import SeverityLabel from '@/components/SeverityLabel.vue'

const route = useRoute()
const traceId = computed(() => String(route.params.id))

const trace = useQuery(
  () => traceId.value,
  async (id) => {
    const spans = await api.trace(id)
    const start = Math.min(...spans.map((s) => s.start))
    const end = Math.max(...spans.map((s) => s.end))
    // Logs of the trace, searched around its time window.
    const window = {
      from: new Date(start / 1e6 - 60_000).toISOString(),
      to: new Date(end / 1e6 + 60_000).toISOString(),
    }
    const logs = await api.logs(window, `trace_id:${id}`, 200).catch(() => [])
    return { spans, logs, start, end }
  },
)

const rows = computed(() => buildWaterfall(trace.data.value?.spans ?? []))
const services = computed(() => [...new Set(rows.value.map((r) => r.span.service))])
const color = (service: string) => `var(--series-${(services.value.indexOf(service) % 8) + 1})`
const root = computed(() => rows.value[0]?.span)
const errors = computed(() => rows.value.filter((r) => r.span.status_code === 'Error').length)
const duration = computed(() => (trace.data.value ? trace.data.value.end - trace.data.value.start : 0))
const ticks = [0, 0.25, 0.5, 0.75, 1]

const selected = ref<Span | null>(null)
</script>

<template>
  <div class="page" :class="{ loading: trace.loading.value }">
    <StatusMessage v-if="trace.error.value" kind="error" title="Could not load the trace" :detail="trace.error.value.message">
      <p><RouterLink to="/traces" class="link">← Back to traces</RouterLink></p>
    </StatusMessage>

    <template v-else-if="trace.data.value && root">
      <header class="summary">
        <RouterLink to="/traces" class="link back">← Traces</RouterLink>
        <h2><span class="svc">{{ root.service }}</span> {{ root.name }}</h2>
        <div class="facts">
          <span>{{ formatDuration(duration) }}</span>
          <span>{{ rows.length }} spans</span>
          <span>{{ services.length }} services</span>
          <span v-if="errors" class="err">⚠ {{ errors }} error{{ errors > 1 ? 's' : '' }}</span>
          <span class="muted">{{ formatDateTime(trace.data.value.start) }}</span>
          <span class="muted mono">{{ traceId }}</span>
        </div>
      </header>

      <div class="card waterfall">
        <div class="axis">
          <div class="axis-name">Service · operation</div>
          <div class="axis-dur">Duration</div>
          <div class="axis-time">
            <span v-for="t in ticks" :key="t" :style="{ left: `${t * 100}%` }">{{ formatDuration(t * duration) }}</span>
          </div>
        </div>
        <div
          v-for="r in rows"
          :key="r.span.span_id"
          class="span-row"
          :class="{ error: r.span.status_code === 'Error', selected: selected === r.span }"
          tabindex="0"
          @click="selected = r.span"
          @keydown.enter="selected = r.span"
        >
          <div class="name" :style="{ paddingLeft: `${r.depth * 16}px` }">
            <span class="dot" :style="{ background: color(r.span.service) }" aria-hidden="true" />
            <span class="svc">{{ r.span.service }}</span>
            <span class="op">{{ r.span.name }}</span>
            <span v-if="r.span.status_code === 'Error'" class="err" aria-label="error">⚠</span>
          </div>
          <div class="dur">{{ formatDuration(r.span.duration_nano) }}</div>
          <div class="timeline">
            <span
              class="bar"
              :style="{ left: `${r.offset * 100}%`, width: `${r.width * 100}%`, background: color(r.span.service) }"
            />
          </div>
        </div>
        <ul class="legend">
          <li v-for="s in services" :key="s"><span class="dot" :style="{ background: color(s) }" />{{ s }}</li>
        </ul>
      </div>

      <section class="logs card">
        <h3>Logs of this trace <span class="muted">({{ trace.data.value.logs.length }})</span></h3>
        <p v-if="trace.data.value.logs.length === 0" class="muted">No logs carry this trace ID.</p>
        <table v-else class="table">
          <tbody>
            <tr v-for="(l, i) in trace.data.value.logs" :key="i">
              <td class="mono muted">{{ formatTime(l.time) }}</td>
              <td><SeverityLabel :level="l.severity" /></td>
              <td>{{ l.service }}</td>
              <td class="mono truncate">{{ l.body }}</td>
            </tr>
          </tbody>
        </table>
      </section>
    </template>

    <SidePanel v-if="selected" :title="`${selected.service} · ${selected.name}`" @close="selected = null">
      <dl class="span-facts">
        <dt>Duration</dt>
        <dd>{{ formatDuration(selected.duration_nano) }}</dd>
        <dt>Kind</dt>
        <dd>{{ selected.kind }}</dd>
        <dt>Status</dt>
        <dd :class="{ err: selected.status_code === 'Error' }">
          {{ selected.status_code }}<template v-if="selected.status_message"> — {{ selected.status_message }}</template>
        </dd>
        <dt>Span ID</dt>
        <dd class="mono">{{ selected.span_id }}</dd>
      </dl>
      <section v-for="(ev, i) in selected.events" :key="i" class="event">
        <AttributeList :title="`Event · ${ev.name}`" :attributes="ev.attributes" />
      </section>
      <AttributeList title="Attributes" :attributes="selected.attributes" />
      <AttributeList title="Resource" :attributes="selected.resource_attributes" />
    </SidePanel>
  </div>
</template>

<style scoped>
.summary {
  margin-bottom: var(--space-3);
}
.back {
  font-size: 12px;
}
h2 {
  margin: var(--space-2) 0;
  font-size: 18px;
  font-weight: 600;
}
.facts {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-4);
}
.link {
  color: var(--accent);
}
.svc {
  font-weight: 600;
}
.err {
  color: var(--status-error);
  font-weight: 600;
}
.axis,
.span-row {
  display: grid;
  grid-template-columns: minmax(220px, 32%) 72px 1fr;
  align-items: center;
}
.axis-dur,
.dur {
  padding-right: var(--space-3);
  text-align: right;
  font-variant-numeric: tabular-nums;
}
.dur {
  color: var(--text-muted);
  font-size: 12px;
}
.axis {
  height: 24px;
  border-bottom: 1px solid var(--border);
  color: var(--text-muted);
  font-size: 11px;
}
.axis-time {
  position: relative;
  height: 100%;
}
.axis-time span {
  position: absolute;
  top: 5px;
  transform: translateX(-50%);
  white-space: nowrap;
}
.axis-time span:first-child {
  transform: none;
}
.axis-time span:last-child {
  transform: translateX(-100%);
}
.span-row {
  height: 28px;
  border-radius: 4px;
  cursor: pointer;
}
.span-row:hover {
  background: var(--bg-hover);
}
.span-row.selected {
  background: var(--accent-soft);
}
.name {
  display: flex;
  align-items: center;
  gap: 6px;
  overflow: hidden;
  white-space: nowrap;
}
.op {
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
}
.dot {
  width: 8px;
  height: 8px;
  border-radius: 2px;
  flex-shrink: 0;
}
.timeline {
  position: relative;
  height: 100%;
}
.bar {
  position: absolute;
  top: 9px;
  height: 10px;
  min-width: 2px;
  border-radius: 2px;
}
.span-row.error .bar {
  outline: 2px solid var(--status-error);
  outline-offset: 1px;
}
.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 14px;
  margin: var(--space-2) 0 0;
  padding: 0;
  list-style: none;
  font-size: 12px;
  color: var(--text-muted);
}
.legend li {
  display: flex;
  align-items: center;
  gap: 6px;
}
.logs h3 {
  margin: 0 0 var(--space-2);
  font-size: 13px;
}
.span-facts {
  display: grid;
  grid-template-columns: 90px 1fr;
  gap: 6px var(--space-3);
  margin: 0 0 var(--space-4);
}
.span-facts dt {
  color: var(--text-muted);
}
.span-facts dd {
  margin: 0;
}
</style>
