<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import type { HistogramBucket } from '@/lib/api'
import { formatCompact } from '@/lib/format'
import { levelColor, levelRank } from '@/lib/severity'
import { niceMax, tickLabel, timeTicks } from './scale'

const props = withDefaults(
  defineProps<{ buckets: HistogramBucket[]; from: number; to: number; height?: number }>(),
  { height: 96 },
)

const PAD = { top: 6, right: 12, bottom: 20, left: 44 }
const root = ref<HTMLElement>()
const width = ref(640)
let observer: ResizeObserver | undefined
onMounted(() => {
  if (!root.value || typeof ResizeObserver === 'undefined') return
  observer = new ResizeObserver(([e]) => {
    if (e) width.value = Math.max(240, e.contentRect.width)
  })
  observer.observe(root.value)
})
onBeforeUnmount(() => observer?.disconnect())

const innerW = computed(() => width.value - PAD.left - PAD.right)
const innerH = computed(() => props.height - PAD.top - PAD.bottom)
const step = computed(() => {
  const ts = props.buckets.map((b) => b.t)
  let min = Infinity
  for (let i = 1; i < ts.length; i++) min = Math.min(min, ts[i]! - ts[i - 1]!)
  return Number.isFinite(min) ? min : (props.to - props.from) / 60
})
const total = (b: HistogramBucket) => Object.values(b.counts).reduce((a, c) => a + c, 0)
const yMax = computed(() => niceMax(Math.max(0, ...props.buckets.map(total))))

const x = (t: number) => PAD.left + ((t - props.from) / Math.max(props.to - props.from, 1)) * innerW.value
const y = (v: number) => PAD.top + innerH.value - (v / yMax.value) * innerH.value
const barW = computed(() => Math.max(1, (step.value / (props.to - props.from)) * innerW.value - 2))

// Most severe at the bottom, so errors sit on the baseline.
const levels = computed(() =>
  [...new Set(props.buckets.flatMap((b) => Object.keys(b.counts)))].sort((a, b) => levelRank(a) - levelRank(b)),
)

const bars = computed(() =>
  props.buckets.map((b) => {
    let acc = 0
    const segments = levels.value
      .filter((l) => (b.counts[l] ?? 0) > 0)
      .map((l) => {
        const v = b.counts[l]!
        const seg = { level: l, y: y(acc + v), h: Math.max(1, y(acc) - y(acc + v) - 1) }
        acc += v
        return seg
      })
    return { bucket: b, x: x(b.t) + 1, segments }
  }),
)

const hover = ref<HistogramBucket | null>(null)
const hoverX = computed(() => (hover.value ? x(hover.value.t) : 0))
const xTicks = computed(() => timeTicks(props.from, props.to, Math.max(2, Math.floor(innerW.value / 110))))
</script>

<template>
  <div ref="root" class="bars">
    <svg :width="width" :height="height" role="img" aria-label="Log volume by severity">
      <g class="grid">
        <line :x1="PAD.left" :x2="width - PAD.right" :y1="y(0)" :y2="y(0)" />
        <text :x="PAD.left - 8" :y="y(yMax)" dy="0.32em" text-anchor="end">{{ formatCompact(yMax) }}</text>
        <text v-for="t in xTicks" :key="t" :x="x(t)" :y="height - 5" text-anchor="middle">
          {{ tickLabel(t, to - from) }}
        </text>
      </g>
      <g
        v-for="bar in bars"
        :key="bar.bucket.t"
        class="bar"
        :class="{ dim: hover && hover !== bar.bucket }"
        @pointerenter="hover = bar.bucket"
        @pointerleave="hover = null"
      >
        <rect class="hit" :x="bar.x - 1" :y="PAD.top" :width="barW + 2" :height="innerH" />
        <rect
          v-for="s in bar.segments"
          :key="s.level"
          :x="bar.x"
          :y="s.y"
          :width="barW"
          :height="s.h"
          :fill="levelColor(s.level)"
          rx="1"
        />
      </g>
    </svg>

    <div v-if="hover" class="tooltip" :style="{ left: `${hoverX}px` }" role="status">
      <div class="muted">{{ tickLabel(hover.t, 0) }}</div>
      <div v-for="l in levels.filter((l) => hover!.counts[l])" :key="l" class="row">
        <span class="key" :style="{ background: levelColor(l) }" />
        <strong>{{ formatCompact(hover.counts[l]!) }}</strong>
        <span class="muted">{{ l }}</span>
      </div>
    </div>

    <ul v-if="levels.length > 1" class="legend">
      <li v-for="l in levels" :key="l">
        <span class="swatch" :style="{ background: levelColor(l) }" />{{ l }}
      </li>
    </ul>
  </div>
</template>

<style scoped>
.bars {
  position: relative;
}
svg {
  display: block;
}
.grid line {
  stroke: var(--grid);
}
.grid text {
  fill: var(--text-muted);
  font-size: 11px;
}
.hit {
  fill: transparent;
}
.bar.dim rect:not(.hit) {
  opacity: 0.45;
}
.tooltip {
  position: absolute;
  top: 0;
  z-index: 5;
  transform: translateX(-50%);
  padding: 6px 8px;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  pointer-events: none;
  font-size: 12px;
  white-space: nowrap;
}
.row {
  display: flex;
  align-items: center;
  gap: 6px;
}
.key {
  width: 12px;
  height: 2px;
}
.muted {
  color: var(--text-muted);
}
.legend {
  display: flex;
  gap: 14px;
  margin: 4px 0 0;
  padding-left: 44px;
  list-style: none;
  font-size: 11px;
  color: var(--text-muted);
}
.legend li {
  display: flex;
  align-items: center;
  gap: 5px;
}
.swatch {
  width: 8px;
  height: 8px;
  border-radius: 2px;
}
</style>
