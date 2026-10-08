<script setup lang="ts">
import { computed } from 'vue'
import { useQuery } from '@/composables/useQuery'
import { api } from '@/lib/api'
import type { TimeRange } from '@/lib/timeRange'
import { formatCompact, formatPercent } from '@/lib/format'
import StatusMessage from './StatusMessage.vue'
import Val from './Val.vue'

/**
 * Explains what distinguishes a selection — the errors, or a period — from
 * the rest of the records: the attribute values over-represented in it.
 */
const props = defineProps<{
  signal: 'logs' | 'spans'
  range: TimeRange
  q: string
  /** A period to compare with the rest; without it, the errors are compared. */
  selection?: { from: string; to: string }
}>()

const result = useQuery(
  () => ({ range: { ...props.range }, q: props.q, sel: props.selection ? { ...props.selection } : null, signal: props.signal }),
  ({ range, q, sel, signal }) =>
    api.compare(range, { signal, q, errors: !sel, selFrom: sel?.from, selTo: sel?.to }),
)
const noun = computed(() => (props.signal === 'spans' ? 'spans' : 'log records'))
const selected = computed(() =>
  props.selection ? `${noun.value} of the selected period` : props.signal === 'spans' ? 'spans in error' : 'error logs',
)
const pct = (v: number) => `${Math.round(v * 100)}%`
</script>

<template>
  <section class="compare" :class="{ loading: result.loading.value }">
    <StatusMessage v-if="result.error.value" kind="error" title="Comparison failed" :detail="result.error.value.message" />
    <template v-else-if="result.data.value">
      <StatusMessage
        v-if="result.data.value.selection_total === 0"
        kind="empty"
        :title="selection ? 'Nothing in the selected period' : `No ${selected} in this period`"
        :detail="selection ? 'Drag across the chart to select another period.' : 'Good news: there is nothing to explain.'"
      />
      <template v-else>
        <p class="headline">
          What distinguishes the <strong>{{ formatCompact(result.data.value.selection_total) }} {{ selected }}</strong>
          from the {{ formatCompact(result.data.value.baseline_total) }} other {{ noun }}
        </p>
        <p v-if="!result.data.value.items.length" class="muted">
          Nothing stands out: the selection looks like the rest. Try narrowing the search.
        </p>
        <div class="legend muted">
          <span><span class="swatch sel" /> in the selection</span>
          <span><span class="swatch base" /> elsewhere</span>
        </div>
        <div v-for="it in result.data.value.items" :key="it.key + it.value" class="row">
          <div class="what">
            <span class="key mono">{{ it.key }}</span>
            <Val :k="it.key" :v="it.value" class="value mono" />
          </div>
          <div class="bars">
            <div class="track"><div class="bar sel" :style="{ width: pct(it.selection) }" /></div>
            <span class="n">{{ formatPercent(it.selection) }}</span>
            <div class="track"><div class="bar base" :style="{ width: pct(it.baseline) }" /></div>
            <span class="n muted">{{ formatPercent(it.baseline) }}</span>
          </div>
        </div>
      </template>
    </template>
  </section>
</template>

<style scoped>
.compare {
  padding: var(--space-4) var(--space-5);
  transition: opacity 120ms;
}
.compare.loading {
  opacity: 0.6;
}
.headline {
  margin: 0 0 var(--space-2);
  font-size: 15px;
}
.legend {
  display: flex;
  gap: var(--space-4);
  margin-bottom: var(--space-3);
  font-size: 12px;
}
.swatch {
  display: inline-block;
  width: 10px;
  height: 10px;
  margin-right: 4px;
  border-radius: 2px;
  vertical-align: -1px;
}
.row {
  display: grid;
  grid-template-columns: minmax(220px, 34%) 1fr;
  align-items: center;
  gap: var(--space-4);
  padding: 10px 0;
  border-bottom: 1px solid var(--border);
}
.what {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.key {
  color: var(--text-muted);
  font-size: 12px;
}
.value {
  font-size: 14px;
  font-weight: 500;
}
.bars {
  display: grid;
  grid-template-columns: 1fr 48px;
  align-items: center;
  gap: 4px var(--space-2);
}
.track {
  height: 8px;
  border-radius: 4px;
  background: var(--bg-hover);
  overflow: hidden;
}
.bar {
  height: 100%;
  border-radius: 4px;
}
.bar.sel,
.swatch.sel {
  background: var(--status-error);
}
.bar.base,
.swatch.base {
  background: var(--text-faint);
}
.n {
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  text-align: right;
}
</style>
