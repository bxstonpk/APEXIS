import type { Metadata } from 'next'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { mfgApi, mfgPage, type ProductionOrder } from '@/features/manufacturing/types'
import { loadItem } from '@/features/organization/pages'
import { tableDocument } from '@/features/print-templates/data'
import { TemplatedPrint } from '@/features/print-templates/TemplatedPrint'
import { printDate } from '@/features/sales/PrintDocument'
import { PrintToolbar } from '@/features/sales/PrintToolbar'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.manufacturing.orders.print.title }

/** พิมพ์ A4 ผ่านแม่แบบ manufacturing.production_order: ใบสั่งผลิต / ใบเบิกวัตถุดิบ — วัตถุดิบที่ต้องใช้ (ช่อง "จ่ายจริง" เขียนมือ) + ลงนาม 3 ฝ่าย */
export default async function Page({ params }: { params: Promise<{ companyId: string; docId: string }> }) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.manufacturing.orders
  const p = t.print
  const f = t.fields
  const res = await loadItem<ProductionOrder>(mfgApi.orders(companyId), docId, t.notFound)
  if ('panel' in res) return res.panel
  const o = res.item
  const data = tableDocument({
    letterhead: null,
    companyName: ctx.company.name,
    copyLabel: '',
    title: p.title,
    docNo: o.document_no ?? t.draftTitle,
    info: [
      [f.date, printDate(o.date)],
      [f.plannedStart, printDate(o.planned_start)],
      [f.plannedEnd, printDate(o.planned_end)],
    ],
    party: {
      label: p.product,
      name: `${o.item.code} ${o.item.name}`,
      extra: [
        `${f.quantity}: ${formatAmount(o.quantity)} ${o.item.uom ?? ''}`.trim(),
        `${f.bom}: ${o.bom.code} ${o.bom.name}`,
        `${f.warehouse}: ${o.warehouse.code} ${o.warehouse.name}`,
      ],
    },
    columns: [
      { label: p.seq, align: 'center', width: '3rem' },
      { label: t.materials.item, align: 'left', width: '' },
      { label: p.unit, align: 'left', width: '5rem' },
      { label: t.materials.required, align: 'right', width: '7rem' },
      { label: p.issuedCol, align: 'left', width: '7rem' },
    ],
    lines: (o.materials ?? []).map((m, i) => ({
      no: i + 1,
      description: `${m.item.code} ${m.item.name}`,
      detail: '',
      quantity: m.required_quantity,
      unit_price: '',
      amount: '',
      cells: [
        { value: String(i + 1), align: 'center' as const },
        { value: `${m.item.code} ${m.item.name}` },
        { value: m.item.uom ?? '' },
        { value: formatAmount(m.required_quantity), align: 'right' as const },
        { value: '', blank: true },
      ],
    })),
    totals: [],
    note: o.description,
    signatures: [p.signOrder, p.signIssue, p.signStore],
  })
  return (
    <>
      <PrintToolbar backHref={`${mfgPage(companyId, 'orders')}/${o.id}`} />
      <TemplatedPrint docType="manufacturing.production_order" data={data} />
    </>
  )
}
