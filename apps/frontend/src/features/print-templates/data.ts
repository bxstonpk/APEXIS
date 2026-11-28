import { format, texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { bahtText } from '@/features/sales/calc'
import type { Letterhead } from '@/features/sales/types'
import type { CompanyPrint, DocumentPrintData, PrintCell } from './types'

/**
 * ข้อมูลพิมพ์ (toPrintData) — แปลงเอกสารเป็น JSON รูปแบบคงที่ให้แม่แบบใช้
 * ชื่อตัวแปรทุกตัวมีคำอธิบายใน catalog.ts (หน้าแก้แม่แบบแสดงให้คลิกแทรก)
 */

export function branchLabel(branchNo: string | undefined): string {
  if (!branchNo) return ''
  return branchNo === '00000' ? texts.sales.print.headOffice : format(texts.sales.print.branch, { no: branchNo })
}

export function letterheadAddress(l: Letterhead): string {
  const a = l.address
  return [a.line, a.sub_district, a.district, a.province, a.postal_code].filter(Boolean).join(' ')
}

/** หัวกระดาษบริษัท — ไม่มีหัวกระดาษ (เช่น ไม่มีสิทธิ์อ่าน) ใช้ชื่อบริษัทอย่างเดียว */
export function companyPrint(l: Letterhead | null, fallbackName = ''): CompanyPrint {
  if (!l) {
    return { name: fallbackName, legal_name_th: fallbackName, legal_name_en: '', tax_id: '', branch_label: '', address: '', phone: '', email: '', website: '', contact: '' }
  }
  return {
    name: l.name,
    legal_name_th: l.legal_name_th || l.name,
    legal_name_en: l.legal_name_en,
    tax_id: l.tax_id,
    branch_label: texts.sales.print.headOffice,
    address: letterheadAddress(l),
    phone: l.phone,
    email: l.email,
    website: l.website,
    contact: [l.phone && `โทร ${l.phone}`, l.email, l.website].filter(Boolean).join(' · '),
  }
}

/** ป้ายข้อความที่แม่แบบใช้ (มาจาก th.ts — แม่แบบไม่ต้องเขียนข้อความคงที่เอง) */
export function printLabels(): Record<string, string> {
  const p = texts.sales.print
  return {
    no: p.no,
    seq: p.seq,
    item: p.itemDesc,
    qty: p.qty,
    unit_price: p.unitPrice,
    amount: p.amount,
    tax_id: p.taxId,
    head_office: p.headOffice,
    sign_date: p.signDate,
    original: p.original,
  }
}

export type PrintParty = { label: string; name: string; taxId?: string; branchNo?: string; address?: string; extra?: string[] }
export type PrintLine = { description: string; detail?: string; quantity?: string; unitPrice?: string; amount: string }
export type PrintTotal = { label: string; value: string; strong?: boolean }

/** เอกสารทั่วไปที่มีรายการแบบ จำนวน × ราคา = จำนวนเงิน (เดิมคือ props ของ PrintDocument) */
export function documentPrintData(d: {
  letterhead: Letterhead | null
  companyName?: string
  title: string
  docNo: string
  info: [string, string][]
  party: PrintParty | null
  lines: PrintLine[]
  totals: PrintTotal[]
  amountInWords?: string
  note?: string
  signatures: string[]
  copyLabel?: string
}): DocumentPrintData {
  const labels = printLabels()
  const priced = d.lines.some((l) => l.unitPrice !== undefined)
  const columns: DocumentPrintData['columns'] = [
    { label: labels.seq!, align: 'center', width: '2.5rem' },
    { label: labels.item!, align: 'left', width: '' },
    ...(priced
      ? ([
          { label: labels.qty!, align: 'right', width: '5rem' },
          { label: labels.unit_price!, align: 'right', width: '7rem' },
        ] as const)
      : []),
    { label: labels.amount!, align: 'right', width: '8rem' },
  ]
  const lines = d.lines.map((l, i) => {
    const cells: PrintCell[] = [
      { value: String(i + 1), align: 'center' },
      { value: l.description, align: 'left', detail: l.detail },
      ...(priced
        ? ([
            { value: l.quantity ? formatAmount(l.quantity) : '', align: 'right' },
            { value: l.unitPrice ? formatAmount(l.unitPrice) : '', align: 'right' },
          ] as PrintCell[])
        : []),
      { value: formatAmount(l.amount), align: 'right' },
    ]
    return { no: i + 1, description: l.description, detail: l.detail ?? '', quantity: l.quantity ?? '', unit_price: l.unitPrice ?? '', amount: l.amount, cells }
  })
  return tableDocument({
    ...d,
    columns,
    lines,
    totals: d.totals.map((t) => ({ label: t.label, value: formatAmount(t.value), raw: t.value, strong: !!t.strong })),
  })
}

/** เอกสารที่ตารางไม่ใช่รูปแบบราคา (ใบสั่งผลิต / ใบสำคัญจ่าย ...) — ผู้เรียกกำหนดคอลัมน์และช่องเอง */
export function tableDocument(d: {
  letterhead: Letterhead | null
  companyName?: string
  title: string
  docNo: string
  info: [string, string][]
  party: PrintParty | null
  columns: DocumentPrintData['columns']
  lines: DocumentPrintData['lines']
  totals: DocumentPrintData['totals']
  amountInWords?: string
  note?: string
  signatures: string[]
  copyLabel?: string
}): DocumentPrintData {
  const words = d.amountInWords ? bahtText(d.amountInWords) : ''
  return {
    company: companyPrint(d.letterhead, d.companyName),
    title: d.title,
    copy_label: d.copyLabel ?? texts.sales.print.original,
    document_no: d.docNo,
    info: d.info.filter(([, v]) => v).map(([label, value]) => ({ label, value })),
    party: d.party
      ? {
          label: d.party.label,
          name: d.party.name,
          tax_id: d.party.taxId ?? '',
          branch_label: branchLabel(d.party.branchNo),
          address: d.party.address ?? '',
          extra: d.party.extra ?? [],
        }
      : null,
    columns: d.columns,
    lines: d.lines,
    totals: d.totals,
    amount_in_words: words.replace(/^\(|\)$/g, ''),
    amount: d.amountInWords ?? '',
    note: d.note ?? '',
    signatures: d.signatures.map((label) => ({ label })),
    labels: printLabels(),
  }
}
