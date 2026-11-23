import { toUnits } from '@/features/accounting/money'

/**
 * คำนวณยอดบรรทัด / ยอดเอกสาร (pure, BigInt สเกล 10^6) — สูตรเดียวกับ backend sales/domain/services.LineAmounts
 * ใช้แสดงตัวอย่างขณะกรอก; backend คำนวณจริงตอนบันทึก (ทศนิยมตามบริษัท)
 */
export type Amounts = { net: bigint; vat: bigint; gross: bigint }

const SCALE = 1_000_000n

/** ปัดครึ่งขึ้นเป็นทศนิยม decimals ตำแหน่ง (หน่วยย่อยสเกล 10^6) */
export function roundUnits(units: bigint, decimals = 2): bigint {
  const q = 10n ** BigInt(6 - decimals)
  if (q === 1n) return units
  return units >= 0n ? ((units + q / 2n) / q) * q : -((-units + q / 2n) / q) * q
}

/** rate = % ของ VAT (0 = ไม่มี VAT), inclusive = ราคารวม VAT แล้ว */
export function lineAmounts(qty: string, price: string, rate: string, inclusive: boolean, decimals = 2): Amounts {
  const base = roundUnits((toUnits(qty) * toUnits(price)) / SCALE, decimals)
  const r = toUnits(rate)
  if (r === 0n) return { net: base, vat: 0n, gross: base }
  if (inclusive) {
    const vat = roundUnits((base * r) / (100n * SCALE + r), decimals)
    return { net: base - vat, vat, gross: base }
  }
  const vat = roundUnits((base * r) / (100n * SCALE), decimals)
  return { net: base, vat, gross: base + vat }
}

export function sumAmounts(list: Amounts[]): Amounts {
  return list.reduce((a, b) => ({ net: a.net + b.net, vat: a.vat + b.vat, gross: a.gross + b.gross }), { net: 0n, vat: 0n, gross: 0n })
}

const DIGITS = ['', 'หนึ่ง', 'สอง', 'สาม', 'สี่', 'ห้า', 'หก', 'เจ็ด', 'แปด', 'เก้า']
const PLACES = ['', 'สิบ', 'ร้อย', 'พัน', 'หมื่น', 'แสน']

/** อ่านจำนวนเต็มไม่เกินหลักล้าน (0 – 999,999) แบบไทย: เอ็ด / ยี่ */
function readGroup(n: number, hasHigher = false): string {
  const s = String(n)
  let out = ''
  for (let i = 0; i < s.length; i++) {
    const d = Number(s[i])
    const place = s.length - i - 1
    if (d === 0) continue
    if (place === 0 && d === 1 && (s.length > 1 || hasHigher)) out += 'เอ็ด'
    else if (place === 1 && d === 1) out += 'สิบ'
    else if (place === 1 && d === 2) out += 'ยี่สิบ'
    else out += DIGITS[d]! + PLACES[place]!
  }
  return out
}

function readInteger(n: bigint): string {
  if (n === 0n) return 'ศูนย์'
  const million = 1_000_000n
  const parts: string[] = []
  let rest = n
  while (rest > 0n) {
    const higher = rest / million
    parts.unshift(readGroup(Number(rest % million), higher > 0n))
    rest = higher
  }
  return parts.map((p, i) => (i < parts.length - 1 ? `${p || ''}ล้าน` : p)).join('')
}

/** จำนวนเงินเป็นตัวอักษรภาษาไทย เช่น "หนึ่งพันเจ็ดสิบบาทถ้วน", "สิบเอ็ดบาทห้าสิบสตางค์" (ปัดสตางค์ 2 ตำแหน่ง) */
export function bahtText(amount: string | bigint): string {
  const units = roundUnits(typeof amount === 'bigint' ? amount : toUnits(amount), 2)
  const neg = units < 0n
  const abs = neg ? -units : units
  const baht = abs / SCALE
  const satang = (abs % SCALE) / 10_000n
  const text = `${readInteger(baht)}บาท${satang === 0n ? 'ถ้วน' : `${readInteger(satang)}สตางค์`}`
  return neg ? `ลบ${text}` : text
}
