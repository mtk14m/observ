import { describe, expect, test } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { useTimeRange } from './useTimeRange'
import { makeRouter } from '@/test/router'

function setup(path: string) {
  let api!: ReturnType<typeof useTimeRange>
  const Probe = defineComponent({
    setup() {
      api = useTimeRange()
      return () => h('div')
    },
  })
  return makeRouter(path).then((router) => {
    mount(Probe, { global: { plugins: [router] } })
    return { api, router }
  })
}

describe('useTimeRange', () => {
  test('defaults to the past hour', async () => {
    const { api } = await setup('/logs')
    expect(api.range.value).toEqual({ from: 'now-1h', to: 'now' })
  })

  test('reads the range from the URL', async () => {
    const { api } = await setup('/logs?from=now-7d&to=now-1d')
    expect(api.range.value).toEqual({ from: 'now-7d', to: 'now-1d' })
  })

  test('ignores an invalid range in the URL', async () => {
    const { api } = await setup('/logs?from=banana&to=now')
    expect(api.range.value).toEqual({ from: 'now-1h', to: 'now' })
  })

  test('writes the range to the URL and keeps other params', async () => {
    const { api, router } = await setup('/logs?q=error')
    api.setRange({ from: 'now-4h', to: 'now' })
    await flushPromises()
    expect(router.currentRoute.value.query).toEqual({ q: 'error', from: 'now-4h', to: 'now' })
    expect(api.range.value).toEqual({ from: 'now-4h', to: 'now' })
  })
})
