import type { Metadata } from 'next'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { mfgApi, mfgPage, type ProductionOrder } from '@/features/manufacturing/types'
import { loadItem } from '@/features/organization/pages'
import { printDate } from '@/features/sales/PrintDocument'
import { PrintToolbar } from '@/features/sales/PrintToolbar'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.manufacturing.orders.print.title }

/** พิมพ์ A4: ใบสั่งผลิต / ใบเบิกวัตถุดิบ — หัวเอกสาร + วัตถุดิบที่ต้องใช้ (ช่อง "จ่ายจริง" เขียนมือ) + ลงนาม 3 ฝ่าย */
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
  const th = 'border-b border-neutral-400 px-2 py-1.5 font-semibold'
  const info: [string, string][] = [
    [f.no, o.document_no ?? t.draftTitle],
    [f.date, printDate(o.date)],
    [f.plannedStart, printDate(o.planned_start)],
    [f.plannedEnd, printDate(o.planned_end)],
  ]
  return (
    <>
      <PrintToolbar backHref={`${mfgPage(companyId, 'orders')}/${o.id}`} />
      <article className="a4-sheet mx-auto flex flex-col bg-white text-[12.5px] leading-snug text-neutral-900 shadow-card print:shadow-none">
        <header className="flex items-start justify-between gap-4 border-b-2 border-neutral-800 pb-4">
          <p className="min-w-0 text-[17px] font-bold">{ctx.company.name}</p>
          <h1 className="shrink-0 text-right text-[20px] font-bold tracking-tight">{p.title}</h1>
        </header>
        <section className="mt-4 grid grid-cols-[1fr_auto] gap-4">
          <div className="rounded border border-neutral-400 p-3">
            <p className="text-[12px] text-neutral-600">{p.product}</p>
            <p className="font-semibold">
              {o.item.code} {o.item.name}
            </p>
            <p>
              {f.quantity}: <span className="font-semibold tabular-nums">{formatAmount(o.quantity)}</span> {o.item.uom}
            </p>
            <p>
              {f.bom}: {o.bom.code} {o.bom.name}
            </p>
            <p>
              {f.warehouse}: {o.warehouse.code} {o.warehouse.name}
            </p>
          </div>
          <dl className="grid min-w-[14rem] grid-cols-[auto_1fr] content-start gap-x-4 gap-y-1 rounded border border-neutral-400 p-3">
            {info
              .filter(([, v]) => v)
              .map(([k, v]) => (
                <div key={k} className="contents">
                  <dt className="text-neutral-600">{k}</dt>
                  <dd className="text-right font-medium tabular-nums">{v}</dd>
                </div>
              ))}
          </dl>
        </section>
        <table className="mt-4 w-full border-collapse">
          <thead>
            <tr className="bg-neutral-100 text-left">
              <th scope="col" className={`${th} w-12 text-center`}>
                {p.seq}
              </th>
              <th scope="col" className={th}>
                {t.materials.item}
              </th>
              <th scope="col" className={`${th} w-20`}>
                {p.unit}
              </th>
              <th scope="col" className={`${th} w-28 text-right`}>
                {t.materials.required}
              </th>
              <th scope="col" className={`${th} w-28 text-right`}>
                {p.issuedCol}
              </th>
            </tr>
          </thead>
          <tbody>
            {(o.materials ?? []).map((m, i) => (
              <tr key={m.line_no} className="break-inside-avoid border-b border-neutral-200 align-top">
                <td className="px-2 py-2 text-center tabular-nums">{i + 1}</td>
                <td className="px-2 py-2">
                  {m.item.code} {m.item.name}
                </td>
                <td className="px-2 py-2">{m.item.uom}</td>
                <td className="px-2 py-2 text-right tabular-nums">{formatAmount(m.required_quantity)}</td>
                <td className="px-2 py-2">
                  <span className="block h-4 border-b border-dotted border-neutral-500" />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {o.description && <p className="mt-3 text-[12px] text-neutral-700">{o.description}</p>}
        <footer className="mt-auto grid break-inside-avoid grid-cols-3 gap-6 pt-14">
          {[p.signOrder, p.signIssue, p.signStore].map((s) => (
            <div key={s} className="text-center">
              <div className="mx-auto h-10 w-44 border-b border-dotted border-neutral-600" />
              <p className="mt-1">{s}</p>
              <p className="text-[12px] text-neutral-600">{p.signDate}</p>
            </div>
          ))}
        </footer>
      </article>
    </>
  )
}
