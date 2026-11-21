import { describe, expect, it } from 'vitest'
import { param, pickQuery, withQuery } from './url'

describe('url helpers', () => {
  it('param: ค่าแรก / ไม่มี = ""', () => {
    expect(param({ q: ['a', 'b'] }, 'q')).toBe('a')
    expect(param({ q: 'x' }, 'q')).toBe('x')
    expect(param({}, 'q')).toBe('')
  })

  it('pickQuery: เฉพาะ key ที่อนุญาตและมีค่า', () => {
    const q = pickQuery({ q: ' hq ', status: '', evil: '1', sort: '-code' }, ['q', 'status', 'sort'])
    expect(q.toString()).toBe('q=hq&sort=-code')
  })

  it('withQuery: แก้ / ลบ key โดยคงค่าอื่นไว้', () => {
    const current = new URLSearchParams('q=hq&page=3')
    expect(withQuery('/x', current, { page: null })).toBe('/x?q=hq')
    expect(withQuery('/x', current, { sort: '-name', page: 1 })).toBe('/x?q=hq&page=1&sort=-name')
    expect(withQuery('/x', {}, {})).toBe('/x')
    expect(current.toString()).toBe('q=hq&page=3') // ไม่แก้ของเดิม
  })
})
