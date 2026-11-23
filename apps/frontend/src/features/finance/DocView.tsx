'use client'

import { Ban, Printer } from 'lucide-react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useState, type ReactNode } from 'react'
import { Alert, Button, ButtonLink } from '@/components'
import { format, texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { FormBar } from '@/features/forms/FormParts'
import { formatDate } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import type { Payment, Receipt, Transfer } from './types'

/** ปุ่มยกเลิกเอกสาร (PATCH status=cancelled) — ยืนยันก่อน, ไม่มีสิทธิ์ / ยกเลิกแล้ว = ไม่แสดง */
function CancelBar({ apiPath, no, canCancel, listHref, printHref }: { apiPath: string; no: string; canCancel: boolean; listHref: string; printHref?: string | null }) {
  const t = texts.finance
  const router = useRouter()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  async function cancel() {
    if (!window.confirm(format(t.actions.cancelConfirm, { no }))) return
    setBusy(true)
    setError(null)
    try {
      await apiRequest(apiPath, { method: 'PATCH', body: { status: 'cancelled' } })
      showToast(format(t.toast.cancelled, { no }))
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }
  return (
    <>
      {error && <Alert tone="error">{error}</Alert>}
      <FormBar>
        {canCancel && (
          <Button variant="danger" loading={busy} loadingText={t.actions.cancelling} onClick={cancel}>
            <Ban aria-hidden />
            {t.actions.cancel}
          </Button>
        )}
        <div className="ml-auto flex gap-2">
          <ButtonLink href={listHref} variant="secondary">
            {texts.common.back}
          </ButtonLink>
          {printHref && (
            <ButtonLink href={printHref} variant="secondary">
              <Printer aria-hidden />
              {texts.sales.actions.print}
            </ButtonLink>
          )}
        </div>
      </FormBar>
    </>
  )
}

function Facts({ items }: { items: [string, ReactNode][] }) {
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

const th = 'px-3 py-2.5 text-left text-[13px] font-medium text-ink-muted'
const num = 'px-3 py-2.5 text-right tabular-nums'

/** ใบรับเงิน / ใบสำคัญจ่าย (ดูอย่างเดียว + ยกเลิก) */
export function MoneyDocView({
  mode,
  doc,
  apiPath,
  listHref,
  entryHref,
  canCancel,
  printHref,
}: {
  mode: 'receipt' | 'payment'
  doc: Receipt | Payment
  apiPath: string
  listHref: string
  entryHref: string | null
  canCancel: boolean
  /** ใบเสร็จรับเงิน: ลิงก์หน้าพิมพ์ A4 */
  printHref?: string | null
}) {
  const t = texts.finance
  const kinds = mode === 'receipt' ? t.kinds.receipt : t.kinds.payment
  const p = doc.party
  const isPayment = mode === 'payment'
  return (
    <div className="flex flex-col gap-6">
      {doc.status === 'cancelled' && <Alert>{t.cancelledNotice}</Alert>}
      <Facts
        items={[
          [t.columns.no, <span className="font-mono tabular-nums">{doc.document_no}</span>],
          [t.fields.date, formatDate(doc.date)],
          [t.fields.kind, kinds[doc.kind] ?? doc.kind],
          [t.columns.party, `${p.code ? `${p.code} · ` : ''}${p.name}`],
          [t.fields.taxId, [p.tax_id, p.branch_no && `${t.fields.branchNo} ${p.branch_no}`].filter(Boolean).join(' · ')],
          [t.fields.description, doc.description],
          [t.actions.openEntry, entryHref ? <Link className="text-primary hover:underline" href={entryHref}>{doc.entry_no}</Link> : ''],
        ]}
      />

      <div className="overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
        <table className="w-full min-w-[40rem] border-collapse text-sm">
          <thead>
            <tr className="border-b border-line bg-subtle">
              <th scope="col" className={th}>
                {t.sections.lines}
              </th>
              <th scope="col" className={`${th} text-right`}>
                {t.fields.amount}
              </th>
              <th scope="col" className={`${th} text-right`}>
                {t.columns.wht}
              </th>
            </tr>
          </thead>
          <tbody>
            {doc.lines.map((l, i) => (
              <tr key={i} className="border-b border-line align-top">
                <td className="px-3 py-2.5">
                  {l.invoice ? (
                    <span className="font-mono text-[13px]">
                      {l.invoice.code}
                      {l.invoice.name && <span className="ml-2 font-sans text-ink-muted">{l.invoice.name}</span>}
                    </span>
                  ) : (
                    l.description || '–'
                  )}
                  {isPayment && 'wht_type' in l && l.wht_type && (
                    <span className="block text-[12px] text-ink-muted">
                      {l.wht_type.name} {l.wht_rate}% · {t.reports.columns.base} {formatAmount(l.wht_base)}
                    </span>
                  )}
                </td>
                <td className={num}>{formatAmount(l.amount)}</td>
                <td className={num}>{formatAmount(l.wht, { dashZero: true })}</td>
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr className="bg-subtle font-semibold">
              <td className="px-3 py-3 text-right text-[13px] text-ink-muted">{t.totals.amount}</td>
              <td className={num}>{formatAmount(doc.amount)}</td>
              <td className={num}>{formatAmount(doc.wht, { dashZero: true })}</td>
            </tr>
          </tfoot>
        </table>
      </div>

      <div className="overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
        <table className="w-full min-w-[32rem] border-collapse text-sm">
          <thead>
            <tr className="border-b border-line bg-subtle">
              <th scope="col" className={th}>
                {t.fields.cashAccount}
              </th>
              <th scope="col" className={th}>
                {t.fields.reference}
              </th>
              <th scope="col" className={`${th} text-right`}>
                {t.fields.amount}
              </th>
            </tr>
          </thead>
          <tbody>
            {doc.methods.map((m, i) => (
              <tr key={i} className="border-b border-line">
                <td className="px-3 py-2.5">
                  {m.cash_account.code} · {m.cash_account.name}
                </td>
                <td className="px-3 py-2.5 text-ink-muted">{m.reference || '–'}</td>
                <td className={num}>{formatAmount(m.amount)}</td>
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr className="bg-subtle font-semibold">
              <td colSpan={2} className="px-3 py-3 text-right text-[13px] text-ink-muted">
                {t.totals.net}
              </td>
              <td className={num}>{formatAmount(doc.net)}</td>
            </tr>
          </tfoot>
        </table>
      </div>

      <CancelBar apiPath={apiPath} no={doc.document_no} canCancel={canCancel && doc.status === 'posted'} listHref={listHref} printHref={printHref} />
    </div>
  )
}

export function TransferView({ doc, apiPath, listHref, entryHref, canCancel }: { doc: Transfer; apiPath: string; listHref: string; entryHref: string | null; canCancel: boolean }) {
  const t = texts.finance
  return (
    <div className="flex flex-col gap-6">
      {doc.status === 'cancelled' && <Alert>{t.cancelledNotice}</Alert>}
      <Facts
        items={[
          [t.columns.no, <span className="font-mono tabular-nums">{doc.document_no}</span>],
          [t.fields.date, formatDate(doc.date)],
          [t.fields.amount, <span className="font-semibold tabular-nums">{formatAmount(doc.amount)}</span>],
          [t.fields.from, `${doc.from.code} · ${doc.from.name}`],
          [t.fields.to, `${doc.to.code} · ${doc.to.name}`],
          [t.fields.reference, doc.reference],
          [t.fields.description, doc.description],
          [t.actions.openEntry, entryHref ? <Link className="text-primary hover:underline" href={entryHref}>{doc.entry_no}</Link> : ''],
        ]}
      />
      <CancelBar apiPath={apiPath} no={doc.document_no} canCancel={canCancel && doc.status === 'posted'} listHref={listHref} />
    </div>
  )
}
