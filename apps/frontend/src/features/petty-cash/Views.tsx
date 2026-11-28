import { Badge } from '@/components'
import { texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { REQUEST_TONES, VOUCHER_TONES, type Totals } from './types'

export function VoucherStatus({ status }: { status: string }) {
  return (
    <Badge tone={VOUCHER_TONES[status] ?? 'neutral'} dot>
      {texts.pettyCash.voucherStatuses[status] ?? status}
    </Badge>
  )
}

export function RequestStatus({ status }: { status: string }) {
  return (
    <Badge tone={REQUEST_TONES[status] ?? 'neutral'} dot>
      {texts.pettyCash.requestStatuses[status] ?? status}
    </Badge>
  )
}

/** สรุปยอดก่อน VAT / VAT / รวม */
export function TotalsCard({ totals, label }: { totals: Totals; label?: string }) {
  const t = texts.pettyCash.vouchers.totals
  return (
    <aside aria-label={label ?? t.gross} className="h-fit rounded-(--radius-panel) border border-line bg-surface p-4 shadow-card">
      <dl className="flex flex-col gap-2 text-sm">
        <div className="flex justify-between gap-4">
          <dt className="text-ink-muted">{t.net}</dt>
          <dd className="tabular-nums">{formatAmount(totals.net)}</dd>
        </div>
        <div className="flex justify-between gap-4">
          <dt className="text-ink-muted">{t.vat}</dt>
          <dd className="tabular-nums">{formatAmount(totals.vat)}</dd>
        </div>
        <div className="flex justify-between gap-4 border-t border-line pt-2 font-semibold">
          <dt>{t.gross}</dt>
          <dd className="tabular-nums">{formatAmount(totals.gross)}</dd>
        </div>
      </dl>
    </aside>
  )
}
