import { describe, expect, test } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import { effectScope, nextTick, ref } from 'vue'
import { useQuery } from './useQuery'

function deferred<T>() {
  let resolve!: (v: T) => void
  let reject!: (e: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function run<T>(fn: () => T): T {
  return effectScope().run(fn)!
}

describe('useQuery', () => {
  test('loads data for the current key', async () => {
    const q = run(() => useQuery(() => 'a', async (k) => k.toUpperCase()))
    expect(q.loading.value).toBe(true)
    await flushPromises()
    expect(q.data.value).toBe('A')
    expect(q.loading.value).toBe(false)
  })

  test('keeps previous data while reloading', async () => {
    const key = ref('a')
    const pending = deferred<string>()
    const q = run(() => useQuery(() => key.value, (k) => (k === 'a' ? Promise.resolve('A') : pending.promise)))
    await flushPromises()

    key.value = 'b'
    await nextTick()
    expect(q.loading.value).toBe(true)
    expect(q.data.value).toBe('A')

    pending.resolve('B')
    await flushPromises()
    expect(q.data.value).toBe('B')
  })

  test('ignores responses that arrive out of order', async () => {
    const key = ref('slow')
    const slow = deferred<string>()
    const q = run(() => useQuery(() => key.value, (k) => (k === 'slow' ? slow.promise : Promise.resolve('fast'))))
    key.value = 'fast'
    await flushPromises()
    slow.resolve('slow')
    await flushPromises()
    expect(q.data.value).toBe('fast')
  })

  test('exposes errors and clears them on success', async () => {
    const key = ref(1)
    const q = run(() =>
      useQuery(() => key.value, async (k) => {
        if (k === 1) throw new Error('boom')
        return 'ok'
      }),
    )
    await flushPromises()
    expect(q.error.value?.message).toBe('boom')

    key.value = 2
    await flushPromises()
    expect(q.error.value).toBeNull()
    expect(q.data.value).toBe('ok')
  })

  test('does not fetch while the key is null', async () => {
    let calls = 0
    const q = run(() => useQuery(() => null, async () => ++calls))
    await flushPromises()
    expect(calls).toBe(0)
    expect(q.loading.value).toBe(false)
  })
})
