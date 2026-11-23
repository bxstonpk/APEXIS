import Link from 'next/link'
import type { ReactNode } from 'react'
import { format, texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { formatDate } from '@/lib/format'
import type { Totals } from './types'

/** ส่วนแสดงผลของเอกสารขาย (server) */

export function Facts({ items }: { items: [string, ReactNode][] }) {
  return (
    <dl className="grid gap-x-8 gap-y-3 rounded-(--radius-panel) border border-line bg-surface p-5 text-sm shadow-card sm:grid-cols-2 lg:grid-cols-3">
      {items.map(([label, value]) => (
        <div key={label} className="min-w-0">
          <dt className="text-[13px] text-ink-muted">{label}</dt>
          <dd className="mt-0.5 break-words">{value || '–'}</dd>
        </div>
      ))}
    </dl>
  )
}

type Row = { key: string | number; item: { code: string; name: string }; description?: string; quantity: string; unitPrice?: string; amount?: string; extra?: ReactNode }

/** ตารางรายการสินค้า (อ่านอย่างเดียว) + ยอดรวม */
export function LinesTable({ rows, totals, vatRate, inclusive }: { rows: Row[]; totals?: Totals; vatRate?: string; inclusive?: boolean }) {
  const tl = texts.sales.lines
  const th = 'px-3 py-2.5 text-left text-[13px] font-medium text-ink-muted'
  const num = 'px-3 py-2.5 text-right tabular-nums'
  const priced = rows.some((r) => r.unitPrice !== undefined)
  return (
    <div className="overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
      <table className="w-full min-w-[40rem] border-collapse text-sm">
        <thead>
          <tr className="border-b border-line bg-subtle">
            <th scope="col" className={th}>
              {tl.item}
            </th>
            <th scope="col" className={`${th} text-right`}>
              {tl.quantity}
            </th>
            {priced && (
              <>
                <th scope="col" className={`${th} text-right`}>
                  {tl.unitPrice}
                </th>
                <th scope="col" className={`${th} text-right`}>
                  {tl.amount}
                </th>
              </>
            )}
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.key} className="border-b border-line align-top">
              <td className="px-3 py-2.5">
                <span className="text-[13px] text-ink-muted">{r.item.code}</span> {r.item.name}
                {r.description && <span className="block text-[12px] text-ink-muted">{r.description}</span>}
                {r.extra}
              </td>
              <td className={num}>{formatAmount(r.quantity)}</td>
              {priced && (
                <>
                  <td className={num}>{formatAmount(r.unitPrice)}</td>
                  <td className={num}>{formatAmount(r.amount)}</td>
                </>
              )}
            </tr>
          ))}
        </tbody>
        {totals && (
          <tfoot className="bg-subtle">
            {(
              [
                [texts.sales.totals.net, totals.net],
                [format(texts.sales.totals.vat, { rate: vatRate ?? '0' }), totals.vat],
                [texts.sales.totals.gross, totals.gross],
              ] as const
            ).map(([label, v], i) => (
              <tr key={label} className={i === 2 ? 'font-semibold' : undefined}>
                <td colSpan={priced ? 3 : 1} className="px-3 py-2 text-right text-[13px] text-ink-muted">
                  {label}
                  {i === 0 && inclusive && <span className="ml-1">({texts.sales.fields.priceIncludesVat})</span>}
                </td>
                <td className="px-3 py-2 text-right tabular-nums">{formatAmount(v)}</td>
              </tr>
            ))}
          </tfoot>
        )}
      </table>
    </div>
  )
}

export function dateText(d: string | null | undefined) {
  return d ? formatDate(d) : ''
}

export function EntryLink({ href, no }: { href: string | null; no: string }) {
  if (!no) return null
  return href ? (
    <Link className="text-primary hover:underline" href={href}>
      {no}
    </Link>
  ) : (
    <>{no}</>
  )
}
