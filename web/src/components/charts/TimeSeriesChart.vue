<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { niceMax, tickLabel, timeTicks } from './scale'

export interface ChartSeries {
  label: string
  points: { t: number; v: number }[]
}

const props = withDefaults(
  defineProps<{
    series: ChartSeries[]
    /** Visible range in unix nanoseconds. */
    from: number
    to: number
    format?: (v: number) => string
    height?: number
  }>(),
  { format: (v: number) => String(v), height: 200 },
)

const PAD = { top: 8, right: 12, bottom: 22, left: 56 }
const root = ref<HTMLElement>()
const width = ref(640)
let observer: ResizeObserver | undefined

onMounted(() => {
  if (!root.value || typeof ResizeObserver === 'undefined') return
  observer = new ResizeObserver(([entry]) => {
    if (entry) width.value = Math.max(240, entry.contentRect.width)
  })
  observer.observe(root.value)
})
onBeforeUnmount(() => observer?.disconnect())

const innerW = computed(() => width.value - PAD.left - PAD.right)
const innerH = computed(() => props.height - PAD.top - PAD.bottom)
const yMax = computed(() => niceMax(Math.max(0, ...props.series.flatMap((s) => s.points.map((p) => p.v)))))

const x = (t: number) => PAD.left + ((t - props.from) / Math.max(props.to - props.from, 1)) * innerW.value
const y = (v: number) => PAD.top + innerH.value - (v / yMax.value) * innerH.value

const yTicks = computed(() => [0, 0.25, 0.5, 0.75, 1].map((f) => f * yMax.value))
const xTicks = computed(() => timeTicks(props.from, props.to, Math.max(2, Math.floor(innerW.value / 110))))

// The data's own interval: the smallest gap between consecutive points.
const interval = computed(() => {
  let min = Infinity
  for (const s of props.series) {
    for (let i = 1; i < s.points.length; i++) min = Math.min(min, s.points[i]!.t - s.points[i - 1]!.t)
  }
  return min
})

// A gap of more than two intervals means missing data: start a new segment.
const paths = computed(() =>
  props.series.map((s) =>
    s.points
      .map((p, i) => {
        const gap = i > 0 && p.t - s.points[i - 1]!.t > 2 * interval.value
        return `${i === 0 || gap ? 'M' : 'L'}${x(p.t).toFixed(1)},${y(p.v).toFixed(1)}`
      })
      .join(''),
  ),
)

// Hover: snap to the nearest timestamp present in any series.
const allTimes = computed(() => [...new Set(props.series.flatMap((s) => s.points.map((p) => p.t)))].sort((a, b) => a - b))
const hoverT = ref<number | null>(null)

function onMove(e: PointerEvent) {
  const rect = (e.currentTarget as Element).getBoundingClientRect()
  const px = e.clientX - rect.left + PAD.left
  let best: number | null = null
  for (const t of allTimes.value) {
    if (best === null || Math.abs(x(t) - px) < Math.abs(x(best) - px)) best = t
  }
  hoverT.value = best
}

const readout = computed(() => {
  if (hoverT.value === null) return []
  return props.series
    .map((s, i) => ({ label: s.label, index: i, point: s.points.find((p) => p.t === hoverT.value) }))
    .filter((r) => r.point)
    .sort((a, b) => b.point!.v - a.point!.v)
})

const tooltipLeft = computed(() => {
  if (hoverT.value === null) return 0
  const px = x(hoverT.value)
  return px > width.value * 0.6 ? px - 12 : px + 12
})
const tooltipFlip = computed(() => hoverT.value !== null && x(hoverT.value) > width.value * 0.6)

const color = (i: number) => `var(--series-${(i % 8) + 1})`
const lastValue = (s: ChartSeries) => s.points.at(-1)?.v
</script>

<template>
  <div ref="root" class="chart">
    <svg
      :width="width"
      :height="height"
      role="img"
      :aria-label="`Chart of ${series.length} series`"
    >
      <g class="grid">
        <g v-for="v in yTicks" :key="v">
          <line :x1="PAD.left" :x2="width - PAD.right" :y1="y(v)" :y2="y(v)" />
          <text :x="PAD.left - 8" :y="y(v)" dy="0.32em" text-anchor="end">{{ format(v) }}</text>
        </g>
        <text v-for="t in xTicks" :key="t" :x="x(t)" :y="height - 6" text-anchor="middle">
          {{ tickLabel(t, to - from) }}
        </text>
      </g>

      <g class="marks">
        <template v-for="(s, i) in series" :key="s.label">
          <path v-if="s.points.length > 1" :d="paths[i]" :stroke="color(i)" class="line" />
          <circle
            v-for="p in s.points.length === 1 ? s.points : []"
            :key="p.t"
            :cx="x(p.t)"
            :cy="y(p.v)"
            r="4"
            :fill="color(i)"
          />
        </template>
      </g>

      <g v-if="hoverT !== null" class="hover">
        <line :x1="x(hoverT)" :x2="x(hoverT)" :y1="PAD.top" :y2="PAD.top + innerH" />
        <circle
          v-for="r in readout"
          :key="r.label"
          :cx="x(hoverT)"
          :cy="y(r.point!.v)"
          r="4"
          :fill="color(r.index)"
          class="ring"
        />
      </g>

      <rect
        class="overlay"
        :x="PAD.left"
        :y="PAD.top"
        :width="innerW"
        :height="innerH"
        @pointermove="onMove"
        @pointerleave="hoverT = null"
      />
    </svg>

    <div
      v-if="hoverT !== null && readout.length"
      class="tooltip"
      :class="{ flip: tooltipFlip }"
      :style="{ left: `${tooltipLeft}px` }"
      role="status"
    >
      <div class="tooltip-time">{{ tickLabel(hoverT, 0) }}</div>
      <div v-for="r in readout" :key="r.label" class="tooltip-row">
        <span class="key" :style="{ background: color(r.index) }" />
        <strong>{{ format(r.point!.v) }}</strong>
        <span class="muted">{{ r.label }}</span>
      </div>
    </div>

    <ul v-if="series.length > 1" class="legend">
      <li v-for="(s, i) in series" :key="s.label">
        <span class="key" :style="{ background: color(i) }" />
        <span class="label">{{ s.label }}</span>
        <span v-if="lastValue(s) !== undefined" class="muted">{{ format(lastValue(s)!) }}</span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.chart {
  position: relative;
  width: 100%;
}
svg {
  display: block;
  overflow: visible;
}
.grid line {
  stroke: var(--grid);
  stroke-width: 1;
}
.grid text {
  fill: var(--text-muted);
  font-size: 11px;
}
.line {
  fill: none;
  stroke-width: 2;
  stroke-linejoin: round;
  stroke-linecap: round;
}
.hover line {
  stroke: var(--text-muted);
  stroke-width: 1;
}
.ring {
  stroke: var(--bg);
  stroke-width: 2;
}
.overlay {
  fill: transparent;
  cursor: crosshair;
}
.tooltip {
  position: absolute;
  top: 4px;
  z-index: 5;
  min-width: 140px;
  padding: 6px 8px;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  pointer-events: none;
  font-size: 12px;
}
.tooltip.flip {
  transform: translateX(-100%);
}
.tooltip-time {
  margin-bottom: 4px;
  color: var(--text-muted);
}
.tooltip-row {
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}
.key {
  display: inline-block;
  width: 12px;
  height: 2px;
  border-radius: 1px;
  flex-shrink: 0;
}
.muted {
  color: var(--text-muted);
}
.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 16px;
  margin: 8px 0 0;
  padding: 0 0 0 56px;
  list-style: none;
  font-size: 12px;
}
.legend li {
  display: flex;
  align-items: center;
  gap: 6px;
}
.label {
  font-family: var(--font-mono);
}
</style>
