<script setup lang="ts">
import { computed } from 'vue'
import type { Edge, ServiceSummary } from '@/lib/api'
import { layoutMap } from '@/lib/serviceMap'
import { formatPercent, formatRate } from '@/lib/format'

const props = defineProps<{ edges: Edge[]; services: ServiceSummary[]; query: Record<string, string> }>()

const NODE_W = 150
const NODE_H = 44
const COL_GAP = 90
const ROW_GAP = 18

const layout = computed(() => layoutMap(props.edges, props.services.map((s) => s.name)))
const stats = computed(() => new Map(props.services.map((s) => [s.name, s])))
const pos = computed(
  () =>
    new Map(
      layout.value.nodes.map((n) => [n.name, { x: n.column * (NODE_W + COL_GAP), y: n.row * (NODE_H + ROW_GAP) }]),
    ),
)
const width = computed(() => layout.value.columns * (NODE_W + COL_GAP) - COL_GAP)
// Edges that skip columns bend below the boxes, into this extra space.
const BEND = NODE_H / 2 + ROW_GAP
const height = computed(() => layout.value.rows * (NODE_H + ROW_GAP) - ROW_GAP + BEND)
const column = computed(() => new Map(layout.value.nodes.map((n) => [n.name, n.column])))
const maxRequests = computed(() => Math.max(1, ...props.edges.map((e) => e.requests)))

const paths = computed(() =>
  props.edges
    .filter((e) => e.from !== e.to)
    .map((e) => {
      const a = pos.value.get(e.from)!
      const b = pos.value.get(e.to)!
      const x1 = a.x + NODE_W
      const y1 = a.y + NODE_H / 2
      const x2 = b.x
      const y2 = b.y + NODE_H / 2
      const skipped = Math.abs(column.value.get(e.to)! - column.value.get(e.from)!) > 1
      const mid = (x1 + x2) / 2
      const d = skipped
        ? `M${x1},${y1} C${x1 + COL_GAP},${y1 + BEND * 1.6} ${x2 - COL_GAP},${y2 + BEND * 1.6} ${x2 - 6},${y2}`
        : `M${x1},${y1} C${mid},${y1} ${mid},${y2} ${x2 - 6},${y2}`
      return {
        edge: e,
        d,
        width: 1 + 3 * Math.sqrt(e.requests / maxRequests.value),
        failing: e.errors / Math.max(e.requests, 1) > 0.01,
      }
    }),
)
</script>

<template>
  <div class="service-map card">
    <svg :viewBox="`-4 -4 ${width + 8} ${height + 8}`" :width="width + 8" :height="height + 8" role="img" aria-label="Service map">
      <defs>
        <marker id="arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
          <path d="M0,0 L10,5 L0,10 z" fill="var(--text-muted)" />
        </marker>
      </defs>
      <path
        v-for="p in paths"
        :key="`${p.edge.from}-${p.edge.to}`"
        class="edge"
        :class="{ failing: p.failing }"
        :d="p.d"
        :stroke-width="p.width"
        marker-end="url(#arrow)"
      >
        <title>{{ p.edge.from }} → {{ p.edge.to }}: {{ p.edge.requests }} calls, {{ p.edge.errors }} errors</title>
      </path>
      <RouterLink
        v-for="n in layout.nodes"
        :key="n.name"
        v-slot="{ href, navigate }"
        :to="{ path: `/services/${n.name}`, query }"
        custom
      >
        <a :href="href" class="node" @click="navigate">
          <rect
            :x="pos.get(n.name)!.x"
            :y="pos.get(n.name)!.y"
            :width="NODE_W"
            :height="NODE_H"
            rx="6"
            :class="{ failing: (stats.get(n.name)?.error_rate ?? 0) > 0.01 }"
          />
          <text :x="pos.get(n.name)!.x + 12" :y="pos.get(n.name)!.y + 18" class="name">{{ n.name }}</text>
          <text v-if="stats.get(n.name)" :x="pos.get(n.name)!.x + 12" :y="pos.get(n.name)!.y + 34" class="meta">
            {{ formatRate(stats.get(n.name)!.rate_per_second) }} · {{ formatPercent(stats.get(n.name)!.error_rate) }} err
          </text>
        </a>
      </RouterLink>
    </svg>
  </div>
</template>

<style scoped>
.service-map {
  overflow-x: auto;
}
svg {
  display: block;
  max-width: 100%;
  height: auto;
}
.edge {
  fill: none;
  stroke: var(--border-strong);
}
.edge.failing {
  stroke: var(--status-error);
  stroke-opacity: 0.7;
}
.node rect {
  fill: var(--bg-elevated);
  stroke: var(--border-strong);
}
.node rect.failing {
  stroke: var(--status-error);
}
.node:hover rect {
  stroke: var(--accent);
}
.name {
  fill: var(--text);
  font-size: 12px;
  font-weight: 600;
}
.meta {
  fill: var(--text-muted);
  font-size: 11px;
}
</style>
