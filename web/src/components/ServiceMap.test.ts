import { expect, test } from 'vitest'
import { mount } from '@vue/test-utils'
import ServiceMap from './ServiceMap.vue'
import { makeRouter } from '@/test/router'

const svc = (name: string) => ({ name, requests: 1, errors: 0, error_rate: 0, rate_per_second: 1, p50_ms: 1, p95_ms: 1, p99_ms: 1 })

test('edges that skip a column bend around the services in between', async () => {
  const router = await makeRouter('/services')
  const w = mount(ServiceMap, {
    props: {
      edges: [
        { from: 'web', to: 'api', requests: 5, errors: 0 },
        { from: 'api', to: 'db', requests: 5, errors: 0 },
        { from: 'web', to: 'db', requests: 1, errors: 0 },
      ],
      services: ['web', 'api', 'db'].map(svc),
      query: {},
    },
    global: { plugins: [router] },
  })
  const skip = w.findAll('path.edge').find((p) => p.find('title').text().startsWith('web → db'))!
  // M x1,y1 C c1x,c1y c2x,c2y x2,y2: the control points leave the row's center line.
  const nums = skip.attributes('d')!.match(/-?\d+(\.\d+)?/g)!.map(Number)
  const [, y1, , c1y] = nums
  expect(c1y).not.toBe(y1)
})
