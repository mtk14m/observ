<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useQuery } from '@/composables/useQuery'
import { api, type Span } from '@/lib/api'
import { buildWaterfall, visibleRows } from '@/lib/trace'
import { formatDateTime, formatDuration, formatTime } from '@/lib/format'
import SidePanel from '@/components/SidePanel.vue'
import AttributeList from '@/components/AttributeList.vue'
import StatusMessage from '@/components/StatusMessage.vue'
import LevelPill from '@/components/LevelPill.vue'
import AppIcon from '@/components/AppIcon.vue'
import Val from '@/components/Val.vue'

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
    return { spans, logs, start, end, window }
  },
)

const rows = computed(() => buildWaterfall(trace.data.value?.spans ?? []))
const collapsed = ref(new Set<string>())
const search = ref('')
const shown = computed(() => visibleRows(rows.value, collapsed.value, search.value))
function toggle(id: string) {
  const next = new Set(collapsed.value)
  if (!next.delete(id)) next.add(id)
  collapsed.value = next
}

const services = computed(() => [...new Set(rows.value.map((r) => r.span.service))])
const color = (service: string) => `var(--series-${(services.value.indexOf(service) % 8) + 1})`
const errors = computed(() => rows.value.filter((r) => r.span.status_code === 'Error').length)
const duration = computed(() => (trace.data.value ? trace.data.value.end - trace.data.value.start : 0))
const ticks = [0, 0.25, 0.5, 0.75, 1]
const logsLink = computed(() =>
  trace.data.value ? { path: '/logs', query: { ...trace.data.value.window, q: `trace_id:${traceId.value}` } } : '/logs',
)

const INDENT = 20
const selected = ref<Span | null>(null)
</script>

<template>
  <div class="trace" :class="{ loading: trace.loading.value }">
    <StatusMessage v-if="trace.error.value" kind="error" title="Could not load the trace" :detail="trace.error.value.message">
      <p><RouterLink to="/traces" class="link">← Back to traces</RouterLink></p>
    </StatusMessage>

    <template v-else-if="trace.data.value && rows.length">
      <section class="summary">
        <span><span class="label">Start date</span>{{ formatDateTime(trace.data.value.start) }}</span>
        <span><span class="label">Max duration</span>{{ formatDuration(duration) }}</span>
        <span><span class="label">Services</span>{{ services.length }}</span>
        <span><span class="label">Total spans</span>{{ rows.length }}</span>
        <span v-if="errors" class="err"><span class="label">Errors</span>{{ errors }}</span>
        <RouterLink :to="logsLink" class="btn view-logs">
          View logs from this trace <AppIcon name="external" :size="14" />
        </RouterLink>
      </section>

      <section class="waterfall">
        <div class="head">
          <label class="search-field">
            <AppIcon name="search" :size="16" />
            <input v-model="search" class="input" placeholder="Search by service or operation" aria-label="Search spans" />
          </label>
          <div class="axis">
            <span v-for="t in ticks" :key="t" :style="{ left: `${t * 100}%` }">{{ formatDuration(t * duration) }}</span>
          </div>
        </div>

        <div class="rows">
          <div class="grid" aria-hidden="true">
            <span v-for="t in ticks.slice(1, -1)" :key="t" :style="{ left: `${t * 100}%` }" />
          </div>
          <div
            v-for="r in shown"
            :key="r.span.span_id"
            class="span-row"
            :class="{ error: r.span.status_code === 'Error', selected: selected === r.span }"
            tabindex="0"
            @click="selected = r.span"
            @keydown.enter="selected = r.span"
          >
            <div class="name" :style="{ paddingLeft: `${r.depth * INDENT}px` }">
              <button
                v-if="r.hasChildren"
                type="button"
                class="toggle"
                :aria-expanded="!collapsed.has(r.span.span_id)"
                :aria-label="collapsed.has(r.span.span_id) ? 'Expand' : 'Collapse'"
                @click.stop="toggle(r.span.span_id)"
              >
                <AppIcon :name="collapsed.has(r.span.span_id) ? 'expand' : 'collapse'" :size="16" />
              </button>
              <span v-else class="toggle-space" />
              <span class="bar-mark" :style="{ background: color(r.span.service) }" aria-hidden="true" />
              <Val k="service.name" :v="r.span.service" class="svc" />
              <span class="op">{{ r.span.name }}</span>
              <span v-if="r.span.status_code === 'Error'" class="err" aria-label="error">⚠</span>
            </div>
            <div class="timeline">
              <span
                class="bar"
                :style="{ left: `${r.offset * 100}%`, width: `${r.width * 100}%`, background: color(r.span.service) }"
              />
              <!-- The timeline keeps a right margin so the label always fits after the bar. -->
              <span class="dur" :style="{ left: `${(r.offset + r.width) * 100}%` }">
                {{ formatDuration(r.span.duration_nano) }}
              </span>
            </div>
          </div>
          <p v-if="!shown.length" class="muted none">No span matches “{{ search }}”.</p>
        </div>
      </section>

      <section class="logs">
        <div class="tabs">
          <span class="tab active">Logs of this trace <span class="count">{{ trace.data.value.logs.length }}</span></span>
        </div>
        <p v-if="trace.data.value.logs.length === 0" class="muted none">No logs carry this trace ID.</p>
        <table v-else class="table">
          <tbody>
            <tr v-for="(l, i) in trace.data.value.logs" :key="i">
              <td class="mono muted">{{ formatTime(l.time) }}</td>
              <td><LevelPill :level="l.severity" /></td>
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
.trace {
  transition: opacity 120ms;
}
.trace.loading {
  opacity: 0.6;
}
.summary {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-6);
  padding: var(--space-3) var(--space-5);
  border-bottom: 1px solid var(--border);
  font-weight: 600;
}
.summary .label {
  margin-right: 6px;
  color: var(--text-muted);
  font-weight: 400;
}
.view-logs {
  margin-left: auto;
}
.err {
  color: var(--status-error);
}
.head,
.span-row {
  display: grid;
  grid-template-columns: minmax(280px, 32%) 1fr;
}
.head {
  align-items: center;
  border-bottom: 1px solid var(--border);
}
.head .search-field {
  padding: var(--space-3) var(--space-3) var(--space-3) var(--space-5);
  border-right: 1px solid var(--border);
}
.head .search-field .icon {
  left: calc(var(--space-5) + 12px);
}
.axis {
  position: relative;
  height: 100%;
  margin: 0 72px 0 var(--space-4);
  color: var(--text-muted);
  font-size: 12px;
}
.axis span {
  position: absolute;
  top: 50%;
  transform: translate(-50%, -50%);
  white-space: nowrap;
}
.axis span:first-child {
  transform: translate(0, -50%);
}
.axis span:last-child {
  transform: translate(-100%, -50%);
}
.rows {
  position: relative;
  padding: var(--space-2) 0;
}
/* Faint vertical guides at each quarter of the timeline. */
.grid {
  position: absolute;
  top: 0;
  bottom: 0;
  left: calc(max(280px, 32%) + var(--space-4));
  right: 72px;
  pointer-events: none;
}
.grid span {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 1px;
  background: var(--grid);
}
.span-row {
  position: relative;
  height: 34px;
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
  gap: 8px;
  margin-left: var(--space-5);
  overflow: hidden;
  white-space: nowrap;
  border-right: 1px solid var(--border);
}
.toggle {
  display: grid;
  place-items: center;
  width: 20px;
  height: 20px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--text-muted);
  cursor: pointer;
}
.toggle:hover {
  color: var(--text);
}
.toggle-space {
  width: 20px;
  flex-shrink: 0;
}
.bar-mark {
  width: 3px;
  height: 18px;
  border-radius: 2px;
  flex-shrink: 0;
}
.svc {
  font-weight: 500;
}
.op {
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
}
.timeline {
  position: relative;
  margin: 0 72px 0 var(--space-4);
}
.bar {
  position: absolute;
  top: 9px;
  height: 16px;
  min-width: 3px;
  border-radius: 3px;
}
.span-row.error .bar {
  box-shadow: 0 0 0 2px var(--bg), 0 0 0 3px var(--status-error);
}
.dur {
  position: absolute;
  top: 8px;
  padding: 0 6px;
  color: var(--text);
  font-size: 13px;
  white-space: nowrap;
}
.none {
  padding: var(--space-3) var(--space-5);
}
.logs {
  margin-top: var(--space-4);
  border-top: 1px solid var(--border);
}
.logs .tabs {
  padding: 0 var(--space-5);
}
.logs .table td:first-child {
  padding-left: var(--space-5);
}
.link {
  color: var(--accent-text);
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
