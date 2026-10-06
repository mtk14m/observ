import { ref, shallowRef, watch, type Ref, type ShallowRef } from 'vue'

export interface Query<T> {
  data: ShallowRef<T | undefined>
  error: Ref<Error | null>
  loading: Ref<boolean>
  refresh: () => Promise<void>
}

/**
 * Fetches data whenever `key` changes. Previous data stays visible while a
 * new request is in flight, and late responses from older keys are dropped.
 * A null key means "nothing to fetch yet".
 */
export function useQuery<K, T>(key: () => K | null, fetcher: (key: K) => Promise<T>): Query<T> {
  const data = shallowRef<T>()
  const error = ref<Error | null>(null)
  const loading = ref(false)
  let generation = 0

  async function load(k: K | null) {
    const current = ++generation
    if (k === null) {
      loading.value = false
      return
    }
    loading.value = true
    try {
      const result = await fetcher(k)
      if (current !== generation) return
      data.value = result
      error.value = null
    } catch (e) {
      if (current !== generation) return
      error.value = e instanceof Error ? e : new Error(String(e))
    } finally {
      if (current === generation) loading.value = false
    }
  }

  watch(key, (k) => void load(k), { immediate: true, deep: true })

  return { data, error, loading, refresh: () => load(key()) }
}
