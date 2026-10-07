import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { SECTIONS } from './sections'
import SectionView from './views/SectionView.vue'
import ServicesView from './views/ServicesView.vue'
import LogsView from './views/LogsView.vue'
import TracesView from './views/TracesView.vue'
import TraceView from './views/TraceView.vue'
import ServiceView from './views/ServiceView.vue'
import AlertsView from './views/AlertsView.vue'
import MetricsView from './views/MetricsView.vue'

const views: Record<string, RouteRecordRaw['component']> = {
  '/services': ServicesView,
  '/traces': TracesView,
  '/logs': LogsView,
  '/metrics': MetricsView,
  '/alerts': AlertsView,
}

const TITLES: Record<string, string> = {
  '/traces': 'Traces explorer',
  '/logs': 'Logs explorer',
  '/metrics': 'Metrics explorer',
}

export const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/services' },
  ...SECTIONS.map((section) => ({
    path: section.path,
    name: section.name,
    component: views[section.path] ?? SectionView,
    props: views[section.path] ? undefined : { section },
    meta: { title: TITLES[section.path] ?? section.name, icon: section.icon, timeless: section.path === '/alerts' },
  })),
  { path: '/traces/:id', name: 'Trace', component: TraceView, meta: { icon: 'traces', parent: { title: 'Traces explorer', path: '/traces' }, param: 'id', short: true, timeless: true } },
  { path: '/services/:name', name: 'Service', component: ServiceView, meta: { icon: 'services', parent: { title: 'Services', path: '/services' }, param: 'name' } },
]

export function createAppRouter() {
  return createRouter({ history: createWebHistory(), routes })
}
