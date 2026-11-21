/**
 * จำนวนเงินของบัญชี — API ส่ง / รับเป็นข้อความทศนิยม (ไม่ใช้ float กันเศษผิด)
 * คำนวณฝั่ง browser ด้วย BigInt สเกล 10^6 (ตรงกับ NUMERIC(24,6) ของ backend)
 */
const SCALE = 6
const FACTOR = 10n ** BigInt(SCALE)
const AMOUNT = /^-?\d+(\.\d+)?$/

/** ข้อความผู้ใช้พิมพ์ (มีจุลภาค / ช่องว่างได้) → ทศนิยมมาตรฐาน หรือ null ถ้าไม่ใช่ตัวเลข; ว่าง = '' */
export function normalizeAmount(raw: string): string | null {
  const s = raw.replace(/[,\s]/g, '')
  if (s === '') return ''
  if (!AMOUNT.test(s)) return null
  if ((s.split('.')[1] ?? '').length > SCALE) return null
  return toString(toUnits(s))
}

/** ทศนิยม → หน่วยย่อย (BigInt) — ข้อความไม่ถูกต้อง / ว่าง = 0 */
export function toUnits(value: string | null | undefined): bigint {
  const s = (value ?? '').replace(/[,\s]/g, '')
  if (!AMOUNT.test(s)) return 0n
  const neg = s.startsWith('-')
  const [int = '0', frac = ''] = s.replace(/^-/, '').split('.')
  const units = BigInt(int) * FACTOR + BigInt((frac + '0'.repeat(SCALE)).slice(0, SCALE))
  return neg ? -units : units
}

/** หน่วยย่อย → ทศนิยม (ตัดศูนย์ท้าย แต่คงอย่างน้อย minFraction ตำแหน่ง) */
export function toString(units: bigint, minFraction = 0): string {
  const neg = units < 0n
  const abs = neg ? -units : units
  const int = abs / FACTOR
  let frac = (abs % FACTOR).toString().padStart(SCALE, '0').replace(/0+$/, '')
  if (frac.length < minFraction) frac = frac.padEnd(minFraction, '0')
  return `${neg ? '-' : ''}${int}${frac ? `.${frac}` : ''}`
}

export function sum(values: readonly (string | null | undefined)[]): bigint {
  return values.reduce((acc, v) => acc + toUnits(v), 0n)
}

const grouping = new Intl.NumberFormat('en-US')

/**
 * แสดงจำนวนเงิน: คั่นหลักพัน, ทศนิยมอย่างน้อย 2 ตำแหน่ง, ติดลบแสดงในวงเล็บ (แบบงบการเงิน)
 * ศูนย์ = "–" เมื่อ dashZero (ช่องเดบิต / เครดิตที่ว่าง)
 */
export function formatAmount(value: string | bigint | null | undefined, { dashZero = false, parens = true } = {}): string {
  const units = typeof value === 'bigint' ? value : toUnits(value)
  if (units === 0n && dashZero) return '–'
  const neg = units < 0n
  const [int = '0', frac = ''] = toString(neg ? -units : units, 2).split('.')
  const text = `${grouping.format(BigInt(int))}.${frac}`
  if (!neg) return text
  return parens ? `(${text})` : `-${text}`
}

/** ทศนิยมสำหรับ CSV (ไม่คั่นหลัก, ลบใช้เครื่องหมาย -) */
export function plainAmount(value: string | null | undefined): string {
  return toString(toUnits(value), 2)
}
