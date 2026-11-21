import { format, texts } from '@/config'

/**
 * รูปแบบเลขที่เอกสาร — ตรวจ / แสดงตัวอย่างฝั่ง browser ขณะพิมพ์
 * กติกาเดียวกับ backend platform/numbering (backend ตรวจซ้ำตอนบันทึกเสมอ)
 *
 *   {prefix} {yyyy} {yy} {bbbb} {bb} {mm} {fy} {seq:N}
 */
export const MAX_LENGTH = 30
export const DEFAULT_TEMPLATE = '{prefix}{fy}-{seq:6}'

const TOKEN = /\{([a-z]+)(?::(\d+))?\}/g
const LITERAL = /^[A-Za-z0-9._/-]*$/
const PREFIX = /^[A-Z0-9._/-]{0,10}$/
const KNOWN = new Set(['prefix', 'yyyy', 'yy', 'bbbb', 'bb', 'mm', 'fy', 'seq'])

type Part = { literal: string } | { token: string; width: number }

export type NumberingContext = { date: Date; fiscalYear: number }

export type ParseResult = { ok: true; parts: Part[] } | { ok: false; error: string }

const t = texts.numbering.invalid

export function parseTemplate(template: string): ParseResult {
  const parts: Part[] = []
  let last = 0
  let seqs = 0
  for (const m of template.matchAll(TOKEN)) {
    const name = m[1]!
    if (m.index > last) parts.push({ literal: template.slice(last, m.index) })
    last = m.index + m[0].length
    if (!KNOWN.has(name)) return { ok: false, error: format(t.unknown, { name }) }
    if (name === 'seq') {
      const width = m[2] === undefined ? 6 : Number(m[2])
      if (width < 1 || width > 10) return { ok: false, error: t.seqWidth }
      seqs++
      parts.push({ token: name, width })
    } else {
      if (m[2] !== undefined) return { ok: false, error: format(t.noWidth, { name }) }
      parts.push({ token: name, width: 0 })
    }
  }
  if (last < template.length) parts.push({ literal: template.slice(last) })
  if (parts.some((p) => 'literal' in p && !LITERAL.test(p.literal))) return { ok: false, error: t.text }
  if (seqs !== 1) return { ok: false, error: t.seqOnce }
  return { ok: true, parts }
}

function render(parts: Part[], prefix: string, c: NumberingContext, seq: number): string {
  const y = c.date.getFullYear()
  const pad = (n: number, w: number) => String(n).padStart(w, '0')
  return parts
    .map((p) => {
      if ('literal' in p) return p.literal
      switch (p.token) {
        case 'prefix':
          return prefix
        case 'yyyy':
          return pad(y, 4)
        case 'yy':
          return pad(y % 100, 2)
        case 'bbbb':
          return pad(y + 543, 4)
        case 'bb':
          return pad((y + 543) % 100, 2)
        case 'mm':
          return pad(c.date.getMonth() + 1, 2)
        case 'fy':
          return String(c.fiscalYear)
        default:
          return seq < 0 ? '9'.repeat(p.width) : pad(seq, p.width)
      }
    })
    .join('')
}

export type Preview = { ok: true; example: string } | { ok: false; error: string }

/**
 * ตรวจ prefix + template แล้วคืนตัวอย่างเลขที่ลำดับ 1 ของวันที่ context
 * (backend แสดงตัวอย่างด้วยปีบัญชี = ปีของวันนี้ — ใช้แบบเดียวกัน)
 */
export function previewNumber(prefix: string, template: string, c: NumberingContext): Preview {
  const p = prefix.trim().toUpperCase()
  if (!PREFIX.test(p)) return { ok: false, error: t.prefix }
  const parsed = parseTemplate(template.trim())
  if (!parsed.ok) return parsed
  const longest = render(parsed.parts, p, { date: new Date(2026, 11, 31), fiscalYear: 2026 }, -1)
  if (longest.length > MAX_LENGTH) return { ok: false, error: format(t.tooLong, { length: longest.length, max: MAX_LENGTH }) }
  return { ok: true, example: render(parsed.parts, p, c, 1) }
}
