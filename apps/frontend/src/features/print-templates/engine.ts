import DOMPurify from 'dompurify'
import Handlebars from 'handlebars'
import { formatAmount } from '@/features/accounting/money'
import { bahtText } from '@/features/sales/calc'
import { formatDate } from '@/lib/format'
import type { Paper } from './types'

/**
 * เครื่องมือแสดงแม่แบบเอกสาร: Handlebars (escape ทุกค่าเป็นค่าเริ่มต้น, ปิดการเข้าถึง prototype) → DOMPurify (ตัดสคริปต์ / event)
 * → iframe sandbox ที่ไม่อนุญาตสคริปต์ (ดู TemplatedPrint)
 *
 * helper ที่มีให้ (ไม่มี helper ที่รันโค้ด):
 *   {{money x}} 1,234.50 · {{qty x}} จำนวน (ทศนิยมตามค่า) · {{date x}} 26 พ.ย. 2569 · {{bahtText x}} (หนึ่งพันบาทถ้วน)
 *   {{#if (eq a b)}} · {{inc @index}} = ลำดับเริ่ม 1 · {{asset "logo"}} = URL ของรูปชื่อ logo
 */

export type AssetMap = Record<string, string>

const hb = Handlebars.create()

hb.registerHelper('money', (v: unknown) => formatAmount(v == null || v === '' ? '0' : String(v)))
hb.registerHelper('qty', (v: unknown) => {
  if (v == null || v === '') return ''
  const s = String(v)
  if (!/^-?\d+(\.\d+)?$/.test(s)) return s
  const [i = '0', f = ''] = s.replace('-', '').split('.')
  const frac = f.replace(/0+$/, '')
  return `${s.startsWith('-') ? '-' : ''}${Number(i).toLocaleString('th-TH')}${frac ? `.${frac}` : ''}`
})
hb.registerHelper('date', (v: unknown) => (typeof v === 'string' ? formatDate(v) : ''))
hb.registerHelper('bahtText', (v: unknown) => (v == null || v === '' ? '' : bahtText(String(v))))
hb.registerHelper('eq', (a: unknown, b: unknown) => a === b)
hb.registerHelper('inc', (v: unknown) => (typeof v === 'number' ? v + 1 : v))
hb.registerHelper('asset', function (this: unknown, name: unknown, options: Handlebars.HelperOptions) {
  const assets = (options?.data?.root as { assets?: AssetMap } | undefined)?.assets ?? {}
  return typeof name === 'string' ? (assets[name] ?? '') : ''
})

/** แปลงแม่แบบเป็น HTML — error ของแม่แบบ (ไวยากรณ์ผิด) โยนต่อให้ผู้เรียกแสดง */
export function renderTemplate(html: string, data: Record<string, unknown>, assets: AssetMap = {}): string {
  const tpl = hb.compile(html, { strict: false, knownHelpersOnly: false })
  return tpl({ ...data, assets }, { allowProtoPropertiesByDefault: false, allowProtoMethodsByDefault: false })
}

/** ตัดสคริปต์ / event handler / URL ที่รันโค้ดออก (ชั้นที่สอง ต่อจากการตรวจของ backend) — ต้องรันใน browser / jsdom */
export function sanitize(html: string): string {
  return DOMPurify.sanitize(html, {
    WHOLE_DOCUMENT: false,
    ADD_TAGS: ['style'],
    FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'form', 'base', 'link', 'meta'],
    FORBID_ATTR: ['srcset'],
    ALLOW_DATA_ATTR: false,
    FORCE_BODY: true,
  })
}

const PAGE: Record<Paper, { size: string; width: string; height: string }> = {
  a4_portrait: { size: 'A4 portrait', width: '210mm', height: '297mm' },
  a4_landscape: { size: 'A4 landscape', width: '297mm', height: '210mm' },
  a5_portrait: { size: 'A5 portrait', width: '148mm', height: '210mm' },
}

const FONT_FACES = [
  ['thai', 400],
  ['thai', 600],
  ['thai', 700],
  ['latin', 400],
  ['latin', 600],
  ['latin', 700],
]
  .map(
    ([subset, w]) =>
      `@font-face{font-family:'IBM Plex Sans Thai';font-style:normal;font-weight:${w};font-display:swap;src:url(/print-fonts/ibm-plex-sans-thai-${subset}-${w}-normal.woff2) format('woff2');${
        subset === 'thai' ? 'unicode-range:U+0E01-0E5B,U+200C-200D,U+25CC;' : ''
      }}`,
  )
  .join('')

/**
 * เอกสาร HTML ทั้งหน้าสำหรับ iframe: ฟอนต์ของระบบ + กระดาษ (@page) + กล่อง .sheet ต่อหน้า
 * แม่แบบเขียนเฉพาะเนื้อหา (ไม่ต้องมี <html> / <body>) — หลายหน้า = หลาย .sheet
 */
export function buildDocument(body: string, paper: Paper): string {
  const pg = PAGE[paper] ?? PAGE.a4_portrait
  const css = `${FONT_FACES}
@page{size:${pg.size};margin:12mm}
*{box-sizing:border-box}
html,body{margin:0;padding:0;background:transparent;color:#171717;font-family:'IBM Plex Sans Thai',system-ui,sans-serif;font-size:12.5px;line-height:1.4;-webkit-print-color-adjust:exact;print-color-adjust:exact}
.sheet{width:${pg.width};min-height:${pg.height};margin:0 auto 16px;padding:12mm;background:#fff;display:flex;flex-direction:column;box-shadow:0 1px 3px rgba(0,0,0,.12),0 8px 24px rgba(15,23,42,.08);break-after:page}
.sheet:last-child{break-after:auto;margin-bottom:0}
@media print{.sheet{margin:0;padding:0;box-shadow:none;min-height:calc(${pg.height} - 24mm)}}
table{border-collapse:collapse}
.num{text-align:right;font-variant-numeric:tabular-nums}`
  return `<!doctype html><html lang="th"><head><meta charset="utf-8"><style>${css}</style></head><body>${body}</body></html>`
}

/** render ครบขั้นตอน: แม่แบบ → sanitize → เอกสารพร้อมกระดาษ */
export function renderDocument(html: string, paper: Paper, data: Record<string, unknown>, assets: AssetMap = {}): string {
  return buildDocument(sanitize(renderTemplate(html, data, assets)), paper)
}
