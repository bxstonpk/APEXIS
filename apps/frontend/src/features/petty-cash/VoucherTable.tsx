import Link from 'next/link'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { formatDate } from '@/lib/format'
import type { VoucherRow } from './types'

const th = 'px-3 py-2.5 text-left text-[13px] font-medium text-ink-muted whitespace-nowrap'

/** ใบสำคัญจ่ายในคำขอเบิกชดเชย (อ่านอย่างเดียว) */
export function VoucherTable({ rows, href, total }: { rows: VoucherRow[]; href: (id: string) => string; total: string }) {
  const c = texts.pettyCash.vouchers.columns
  return (
    <section aria-labelledby="pc-vouchers" className="flex min-w-0 flex-col gap-3">
      <h2 id="pc-vouchers" className="text-[15px] font-semibold">
        {texts.pettyCash.requests.fields.vouchers}
      </h2>
      <div className="overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
        <table className="w-full min-w-[40rem] border-collapse text-sm">
          <thead>
            <tr className="border-b border-line bg-subtle">
              <th scope="col" className={th}>{c.date}</th>
              <th scope="col" className={th}>{c.no}</th>
              <th scope="col" className={th}>{c.payee}</th>
              <th scope="col" className={th}>{c.taxInvoice}</th>
              <th scope="col" className={`${th} text-right`}>{c.gross}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((v) => (
              <tr key={v.id} className="border-b border-line align-top">
                <td className="px-3 py-2 whitespace-nowrap">{formatDate(v.date)}</td>
                <td className="px-3 py-2">
                  <Link href={href(v.id)} className="font-mono text-[13px] text-primary hover:underline">
                    {v.document_no}
                  </Link>
                </td>
                <td className="px-3 py-2">
                  <span className="block">{v.payee_name}</span>
                  {v.description && <span className="block text-[12px] text-ink-muted">{v.description}</span>}
                </td>
                <td className="px-3 py-2 whitespace-nowrap text-ink-muted">{v.tax_invoice_no || '–'}</td>
                <td className="px-3 py-2 text-right tabular-nums">{formatAmount(v.totals.gross)}</td>
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr className="font-semibold">
              <td colSpan={4} className="px-3 py-2.5 text-right">
                {texts.pettyCash.requests.print.total}
              </td>
              <td className="px-3 py-2.5 text-right tabular-nums">{formatAmount(total)}</td>
            </tr>
          </tfoot>
        </table>
      </div>
    </section>
  )
}
