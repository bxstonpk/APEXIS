import { describe, expect, it } from 'vitest'
import { toUnits } from '@/features/accounting/money'
import { bahtText, lineAmounts } from './calc'

const u = (s: string) => toUnits(s)

describe('lineAmounts (สูตรเดียวกับ backend)', () => {
  it('ราคาแยก VAT: VAT = ฐาน × 7%', () => {
    expect(lineAmounts('3', '100', '7', false)).toEqual({ net: u('300'), vat: u('21'), gross: u('321') })
  })
  it('ราคารวม VAT: VAT = ฐาน × 7 / 107 (ปัด 2 ตำแหน่ง)', () => {
    expect(lineAmounts('1', '1070', '7', true)).toEqual({ net: u('1000'), vat: u('70'), gross: u('1070') })
    expect(lineAmounts('1', '100', '7', true)).toEqual({ net: u('93.46'), vat: u('6.54'), gross: u('100') })
  })
  it('ไม่จด VAT: ไม่มีภาษี', () => {
    expect(lineAmounts('2', '50.5', '0', false)).toEqual({ net: u('101'), vat: 0n, gross: u('101') })
  })
})

describe('bahtText', () => {
  it.each([
    ['0', 'ศูนย์บาทถ้วน'],
    ['1', 'หนึ่งบาทถ้วน'],
    ['11', 'สิบเอ็ดบาทถ้วน'],
    ['21.50', 'ยี่สิบเอ็ดบาทห้าสิบสตางค์'],
    ['1070', 'หนึ่งพันเจ็ดสิบบาทถ้วน'],
    ['1000001', 'หนึ่งล้านเอ็ดบาทถ้วน'],
    ['2500000.25', 'สองล้านห้าแสนบาทยี่สิบห้าสตางค์'],
  ])('%s → %s', (n, words) => {
    expect(bahtText(n)).toBe(words)
  })
})
