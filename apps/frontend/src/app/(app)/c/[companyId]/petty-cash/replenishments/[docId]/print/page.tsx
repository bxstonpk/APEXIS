import type { Metadata } from 'next'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { loadItem } from '@/features/organization/pages'
import { pcApi, pcPage, type Replenishment } from '@/features/petty-cash/types'
import { bahtText } from '@/features/sales/calc'
import { printDate } from '@/features/sales/PrintDocument'
import { PrintToolbar } from '@/features/sales/PrintToolbar'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.pettyCash.requests.print.title }

/** พิมพ์ A4: ใบขอเบิกชดเชยเงินสดย่อย — รายการใบสำคัญจ่ายที่แนบ + ยอดรวม + ลงนาม ผู้ขอเบิก / ผู้อนุมัติ / การเงิน */
export default async function Page({ params }: { params: Promise<{ companyId: string; docId: string }> }) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.pettyCash.requests
  const p = t.print
  const f = t.fields
  const c = texts.pettyCash.vouchers.columns
  const res = await loadItem<Replenishment>(pcApi.requests(companyId), docId, t.notFound)
  if ('panel' in res) return res.panel
  const r = res.item
  const th = 'border-b border-neutral-400 px-2 py-1.5 font-semibold'
  const info: [string, string][] = [
    [f.no, r.document_no],
    [f.date, printDate(r.date)],
    [f.kind, texts.pettyCash.requestKinds[r.kind] ?? r.kind],
  ]
  return (
    <>
      <PrintToolbar backHref={`${pcPage(companyId, 'replenishments')}/${r.id}`} />
      <article className="a4-sheet mx-auto flex flex-col bg-white text-[12.5px] leading-snug text-neutral-900 shadow-card print:shadow-none">
        <header className="flex items-start justify-between gap-4 border-b-2 border-neutral-800 pb-4">
          <p className="min-w-0 text-[17px] font-bold">{ctx.company.name}</p>
          <h1 className="shrink-0 text-right text-[20px] font-bold tracking-tight">{p.title}</h1>
        </header>
        <section className="mt-4 grid grid-cols-[1fr_auto] gap-4">
          <div className="rounded border border-neutral-400 p-3">
            <p className="text-[12px] text-neutral-600">{f.fund}</p>
            <p className="font-semibold">
              {r.fund.code} {r.fund.name}
            </p>
            {r.description && <p className="mt-1">{r.description}</p>}
          </div>
          <dl className="grid min-w-[14rem] grid-cols-[auto_1fr] content-start gap-x-4 gap-y-1 rounded border border-neutral-400 p-3">
            {info.map(([k, v]) => (
              <div key={k} className="contents">
                <dt className="text-neutral-600">{k}</dt>
                <dd className="text-right font-medium tabular-nums">{v}</dd>
              </div>
            ))}
          </dl>
        </section>
        {r.kind === 'replenish' && (
          <table className="mt-4 w-full border-collapse">
            <thead>
              <tr className="bg-neutral-100 text-left">
                <th scope="col" className={`${th} w-12 text-center`}>
                  {p.seq}
                </th>
                <th scope="col" className={`${th} w-24`}>
                  {c.date}
                </th>
                <th scope="col" className={`${th} w-32`}>
                  {c.no}
                </th>
                <th scope="col" className={th}>
                  {c.payee}
                </th>
                <th scope="col" className={`${th} w-32 text-right`}>
                  {c.gross}
                </th>
              </tr>
            </thead>
            <tbody>
              {r.vouchers.map((v, i) => (
                <tr key={v.id} className="break-inside-avoid border-b border-neutral-200 align-top">
                  <td className="px-2 py-2 text-center tabular-nums">{i + 1}</td>
                  <td className="px-2 py-2 tabular-nums">{printDate(v.date)}</td>
                  <td className="px-2 py-2">{v.document_no}</td>
                  <td className="px-2 py-2">
                    {v.payee_name}
                    {v.description && <span className="block text-[11.5px] text-neutral-600">{v.description}</span>}
                  </td>
                  <td className="px-2 py-2 text-right tabular-nums">{formatAmount(v.totals.gross)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        <section className="mt-3 flex items-center justify-between gap-6 break-inside-avoid">
          <p className="min-w-0 flex-1 rounded bg-neutral-100 px-3 py-2 text-center font-semibold">({bahtText(r.amount)})</p>
          <p className="w-64 text-right">
            <span className="font-semibold">{p.total}</span> <span className="ml-4 font-semibold tabular-nums">{formatAmount(r.amount)}</span>
          </p>
        </section>
        <footer className="mt-auto grid break-inside-avoid grid-cols-3 gap-6 pt-14">
          {[p.signRequester, p.signApprover, p.signFinance].map((s) => (
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
