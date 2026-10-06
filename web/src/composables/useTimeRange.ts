import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { DEFAULT_RANGE, resolve, type TimeRange } from '@/lib/timeRange'

/**
 * The global time range, stored in the URL (`?from=…&to=…`) so every view
 * can be shared as a link.
 */
export function useTimeRange() {
  const route = useRoute()
  const router = useRouter()

  const range = computed<TimeRange>(() => {
    const { from, to } = route.query
    if (typeof from === 'string' && typeof to === 'string' && resolve({ from, to }, new Date())) {
      return { from, to }
    }
    return DEFAULT_RANGE
  })

  /** The range as URL query parameters, to carry it across links. */
  const query = computed<Record<string, string>>(() => ({ from: range.value.from, to: range.value.to }))

  function setRange(next: TimeRange) {
    return router.push({ query: { ...route.query, from: next.from, to: next.to } })
  }

  return { range, query, setRange }
}
