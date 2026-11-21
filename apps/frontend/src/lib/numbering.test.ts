import { describe, expect, it } from 'vitest'
import { texts } from '@/config'
import { DEFAULT_TEMPLATE, previewNumber } from './numbering'

const ctx = { date: new Date(2026, 10, 21), fiscalYear: 2026 }

describe('previewNumber (กติกาเดียวกับ backend platform/numbering)', () => {
  it.each([
    ['PV', DEFAULT_TEMPLATE, 'PV2026-000001'],
    ['AJ', '{prefix}{yy}{mm}-{seq:5}', 'AJ2611-00001'], // ตรงกับตัวอย่างที่ backend คืน
    ['iv', '{prefix}{bbbb}/{seq:4}', 'IV2569/0001'],
    ['', '{bb}{mm}{seq}', '6911000001'],
  ])('%s + %s = %s', (prefix, template, example) => {
    expect(previewNumber(prefix, template, ctx)).toEqual({ ok: true, example })
  })

  it.each([
    ['PV', '{prefix}{fy}', texts.numbering.invalid.seqOnce],
    ['PV', '{seq}{seq}', texts.numbering.invalid.seqOnce],
    ['PV', '{prefix}{seq:11}', texts.numbering.invalid.seqWidth],
    ['PV', '{yyyy:2}{seq}', 'ตัวแปร {yyyy} ไม่รับจำนวนหลัก'],
    ['PV', '{day}{seq}', 'ไม่รู้จักตัวแปร {day}'],
    ['PV', 'PV #{seq}', texts.numbering.invalid.text],
    ['TOO-LONG-PREFIX', '{prefix}{seq}', texts.numbering.invalid.prefix],
  ])('ผิด: %s + %s', (prefix, template, error) => {
    expect(previewNumber(prefix, template, ctx)).toEqual({ ok: false, error })
  })

  it('ยาวเกิน 30 ตัว', () => {
    const r = previewNumber('ABCDEFGHIJ', '{prefix}{yyyy}{bbbb}{mm}-{seq:10}', ctx)
    expect(r.ok).toBe(false)
  })
})
