import type { Metadata } from 'next'
import { texts } from '@/config'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { loadItem } from '@/features/organization/pages'
import { pcApi, pcPage, type Voucher } from '@/features/petty-cash/types'
import { bahtText } from '@/features/sales/calc'
import { printDate } from '@/features/sales/PrintDocument'
import { PrintToolbar } from '@/features/sales/PrintToolbar'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.pettyCash.vouchers.print.title }

/** พิมพ์ A4: ใบสำคัญจ่ายเงินสดย่อย — ผู้รับเงิน / รายการ / ยอด VAT / จำนวนเงินตัวอักษร / ลงนาม 3 ฝ่าย */
export default async function Page({ params }: { params: Promise<{ companyId: string; docId: string }> }) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.pettyCash.vouchers
  const p = t.print
  const f = t.fields
  const res = await loadItem<Voucher>(pcApi.vouchers(companyId), docId, t.notFound)
  if ('panel' in res) return res.panel
  const v = res.item
  const hasVat = toUnits(v.vat_rate) > 0n
  const th = 'border-b border-neutral-400 px-2 py-1.5 font-semibold'
  const info: [string, string][] = [
    [f.no, v.document_no],
    [f.date, printDate(v.date)],
    [f.fund, v.fund.code],
    ...(hasVat && v.tax_invoice
      ? ([
          [f.taxInvoiceNo, v.tax_invoice.no],
          [f.taxInvoiceDate, printDate(v.tax_invoice.date)],
        ] as [string, string][])
      : []),
  ]
  const totals: [string, string][] = hasVat
    ? [
        [t.totals.net, v.totals.net],
        [`${t.totals.vat} ${formatAmount(v.vat_rate)}%`, v.totals.vat],
        [t.totals.gross, v.totals.gross],
      ]
    : [[t.totals.gross, v.totals.gross]]
  return (
    <>
      <PrintToolbar backHref={`${pcPage(companyId, 'vouchers')}/${v.id}`} />
      <article className="a4-sheet mx-auto flex flex-col bg-white text-[12.5px] leading-snug text-neutral-900 shadow-card print:shadow-none">
        <header className="flex items-start justify-between gap-4 border-b-2 border-neutral-800 pb-4">
          <p className="min-w-0 text-[17px] font-bold">{ctx.company.name}</p>
          <h1 className="shrink-0 text-right text-[20px] font-bold tracking-tight">{p.title}</h1>
        </header>
        <section className="mt-4 grid grid-cols-[1fr_auto] gap-4">
          <div className="rounded border border-neutral-400 p-3">
            <p className="text-[12px] text-neutral-600">{f.payee}</p>
            <p className="font-semibold">{v.payee.name}</p>
            {v.payee.tax_id && (
              <p>
                {f.taxId}: {v.payee.tax_id}
                {v.payee.branch_no && ` · ${f.branchNo} ${v.payee.branch_no}`}
              </p>
            )}
            {v.description && <p className="mt-1">{v.description}</p>}
          </div>
          <dl className="grid min-w-[14rem] grid-cols-[auto_1fr] content-start gap-x-4 gap-y-1 rounded border border-neutral-400 p-3">
            {info.map(([k, val]) => (
              <div key={k} className="contents">
                <dt className="text-neutral-600">{k}</dt>
                <dd className="text-right font-medium tabular-nums">{val}</dd>
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
                {f.lineDescription}
              </th>
              <th scope="col" className={`${th} w-48`}>
                {f.account}
              </th>
              <th scope="col" className={`${th} w-32 text-right`}>
                {f.amount}
              </th>
            </tr>
          </thead>
          <tbody>
            {v.lines.map((l, i) => (
              <tr key={l.line_no} className="break-inside-avoid border-b border-neutral-200 align-top">
                <td className="px-2 py-2 text-center tabular-nums">{i + 1}</td>
                <td className="px-2 py-2">{l.description}</td>
                <td className="px-2 py-2">
                  {l.account.code} {l.account.name}
                </td>
                <td className="px-2 py-2 text-right tabular-nums">{formatAmount(l.amount)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <section className="mt-3 flex items-start justify-between gap-6 break-inside-avoid">
          <div className="min-w-0 flex-1">
            <p className="text-[12px] text-neutral-600">{p.amountWords}</p>
            <p className="rounded bg-neutral-100 px-3 py-2 text-center font-semibold">({bahtText(v.totals.gross)})</p>
          </div>
          <dl className="grid w-64 grid-cols-[1fr_auto] gap-x-4 gap-y-1">
            {totals.map(([k, val], i) => (
              <div key={k} className="contents">
                <dt className={i === totals.length - 1 ? 'font-semibold' : 'text-neutral-600'}>{k}</dt>
                <dd className={`text-right tabular-nums ${i === totals.length - 1 ? 'font-semibold' : ''}`}>{formatAmount(val)}</dd>
              </div>
            ))}
          </dl>
        </section>
        <footer className="mt-auto grid break-inside-avoid grid-cols-3 gap-6 pt-14">
          {[p.signPayee, p.signCustodian, p.signApprover].map((s) => (
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
