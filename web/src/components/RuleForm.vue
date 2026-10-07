<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { api, type AlertRule, type Channel, type MetricInfo, type PreviewValue } from '@/lib/api'
import { aggregations } from '@/lib/metrics'
import { groupLabel } from '@/lib/alerts'

const props = defineProps<{ rule?: AlertRule; channels: Channel[]; metrics: MetricInfo[] }>()
const emit = defineEmits<{ saved: []; deleted: [] }>()

const draft = reactive<AlertRule>(
  props.rule
    ? structuredClone({ ...props.rule, channels: [...props.rule.channels], group_by: [...(props.rule.group_by ?? [])] })
    : {
        name: '', kind: 'logs', query: '', metric: '', agg: '', group_by: [], op: '>', threshold: 0,
        window_seconds: 300, for_seconds: 0, channels: [], enabled: true,
      },
)
const groupBy = ref((draft.group_by ?? []).join(', '))
const error = ref('')
const saving = ref(false)
const preview = ref<PreviewValue[] | null>(null)

const metric = computed(() => props.metrics.find((m) => m.name === draft.metric))
const aggOptions = computed(() => (metric.value ? aggregations(metric.value) : []))
const windows = [
  { s: 60, label: '1 minute' },
  { s: 300, label: '5 minutes' },
  { s: 900, label: '15 minutes' },
  { s: 3600, label: '1 hour' },
]
const waits = [
  { s: 0, label: 'immediately' },
  { s: 60, label: 'after 1 minute' },
  { s: 300, label: 'after 5 minutes' },
  { s: 900, label: 'after 15 minutes' },
]

function payload(): AlertRule {
  const r: AlertRule = {
    ...draft,
    threshold: Number(draft.threshold),
    window_seconds: Number(draft.window_seconds),
    for_seconds: Number(draft.for_seconds),
    group_by: groupBy.value.split(',').map((s) => s.trim()).filter(Boolean),
  }
  if (r.kind === 'logs') {
    delete r.metric
    delete r.agg
  } else {
    delete r.query
  }
  delete r.id
  delete r.created_at
  delete r.updated_at
  return r
}

async function run(action: () => Promise<void>) {
  error.value = ''
  saving.value = true
  try {
    await action()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    saving.value = false
  }
}

const doPreview = () =>
  run(async () => {
    preview.value = await api.previewRule(payload())
  })
const save = () =>
  run(async () => {
    if (props.rule?.id) await api.updateRule(props.rule.id, payload())
    else await api.createRule(payload())
    emit('saved')
  })
const remove = () =>
  run(async () => {
    await api.deleteRule(props.rule!.id!)
    emit('deleted')
  })
</script>

<template>
  <form class="rule-form" @submit.prevent="save">
    <label class="field">
      <span>Name</span>
      <input v-model="draft.name" name="name" class="input" placeholder="Checkout errors" />
    </label>

    <fieldset class="field">
      <legend>Watch</legend>
      <div class="segmented">
        <label><input v-model="draft.kind" type="radio" name="kind" value="logs" /> Log count</label>
        <label><input v-model="draft.kind" type="radio" name="kind" value="metric" /> Metric</label>
      </div>
    </fieldset>

    <label v-if="draft.kind === 'logs'" class="field">
      <span>Logs matching</span>
      <input v-model="draft.query" name="query" class="input mono" placeholder='service:checkout level:error' />
    </label>
    <template v-else>
      <label class="field">
        <span>Metric</span>
        <select v-model="draft.metric" name="metric" class="select">
          <option value="" disabled>Choose a metric</option>
          <option v-for="m in metrics" :key="m.name" :value="m.name">{{ m.name }}</option>
        </select>
      </label>
      <label class="field">
        <span>Aggregation</span>
        <select v-model="draft.agg" name="agg" class="select">
          <option value="">default</option>
          <option v-for="o in aggOptions" :key="o.value" :value="o.value">{{ o.label }}</option>
        </select>
      </label>
    </template>

    <label class="field">
      <span>One alert per <small class="muted">(optional, comma-separated attributes)</small></span>
      <input v-model="groupBy" name="group_by" class="input mono" placeholder="service.name" />
    </label>

    <div class="field condition">
      <span>Alert when the value is</span>
      <div class="row">
        <select v-model="draft.op" name="op" class="select" aria-label="Operator">
          <option value=">">above</option>
          <option value=">=">at least</option>
          <option value="<">below</option>
          <option value="<=">at most</option>
        </select>
        <input v-model="draft.threshold" name="threshold" type="number" step="any" class="input" aria-label="Threshold" />
        <span class="muted">over the last</span>
        <select v-model.number="draft.window_seconds" name="window" class="select" aria-label="Window">
          <option v-for="w in windows" :key="w.s" :value="w.s">{{ w.label }}</option>
        </select>
      </div>
      <div class="row">
        <span class="muted">Fire</span>
        <select v-model.number="draft.for_seconds" name="for" class="select" aria-label="Wait before firing">
          <option v-for="w in waits" :key="w.s" :value="w.s">{{ w.label }}</option>
        </select>
      </div>
    </div>

    <fieldset class="field">
      <legend>Notify</legend>
      <p v-if="!channels.length" class="muted">No channels yet: add one in the Channels panel.</p>
      <label v-for="c in channels" :key="c.id" class="check">
        <input v-model="draft.channels" type="checkbox" :value="c.id" /> {{ c.name }} <span class="muted">({{ c.type }})</span>
      </label>
    </fieldset>

    <label class="check"><input v-model="draft.enabled" type="checkbox" name="enabled" /> Enabled</label>

    <div v-if="preview" class="preview-result">
      <strong>Current value</strong>
      <p v-if="!preview.length" class="muted">No data in the window.</p>
      <div v-for="(v, i) in preview" :key="i" class="preview-row">
        <span class="mono">{{ groupLabel(v.labels) }}</span>
        <span class="value">{{ Number(v.value.toPrecision(4)) }}</span>
        <span :class="v.breached ? 'breach' : 'ok'">{{ v.breached ? '⚠ BREACH' : '✓ OK' }}</span>
      </div>
    </div>

    <p v-if="error" class="error" role="alert">{{ error }}</p>

    <div class="actions">
      <button type="button" class="btn preview" :disabled="saving" @click="doPreview">Preview</button>
      <button v-if="rule?.id" type="button" class="btn danger" :disabled="saving" @click="remove">Delete</button>
      <button type="submit" class="btn primary" :disabled="saving">Save</button>
    </div>
  </form>
</template>

<style scoped>
.rule-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}
.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin: 0;
  padding: 0;
  border: 0;
}
.field > span,
legend {
  padding: 0;
  font-size: 12px;
  font-weight: 600;
}
.segmented {
  display: flex;
  gap: var(--space-4);
}
.row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
}
.row .input {
  width: 110px;
}
.preview-result {
  padding: var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius);
}
.preview-row {
  display: grid;
  grid-template-columns: 1fr auto auto;
  gap: var(--space-3);
  margin-top: 4px;
}
.value {
  font-variant-numeric: tabular-nums;
  font-weight: 600;
}
.breach {
  color: var(--status-error);
  font-weight: 600;
}
.ok {
  color: var(--status-ok);
}
.error {
  margin: 0;
  color: var(--status-error);
}
.actions {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
}
.actions .preview {
  margin-right: auto;
}
</style>
