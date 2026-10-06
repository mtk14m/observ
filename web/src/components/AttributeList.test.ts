import { expect, test } from 'vitest'
import { mount } from '@vue/test-utils'
import AttributeList from './AttributeList.vue'

test('lists attributes sorted by key, and hides when empty', () => {
  const w = mount(AttributeList, { props: { title: 'Attributes', attributes: { b: '2', a: '<script>' } } })
  expect(w.findAll('dt').map((d) => d.text())).toEqual(['a', 'b'])
  expect(w.find('dd').text()).toBe('<script>') // rendered as text, never HTML

  const empty = mount(AttributeList, { props: { title: 'Attributes', attributes: {} } })
  expect(empty.find('section').exists()).toBe(false)
})
