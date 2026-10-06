import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { SECTIONS } from './sections'
import SectionView from './views/SectionView.vue'

export const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/services' },
  ...SECTIONS.map((section) => ({
    path: section.path,
    name: section.name,
    component: SectionView,
    props: { section },
    meta: { title: section.name },
  })),
]

export function createAppRouter() {
  return createRouter({ history: createWebHistory(), routes })
}
