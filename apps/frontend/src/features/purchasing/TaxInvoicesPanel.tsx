'use client'

import { FileCheck2, X } from 'lucide-react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useEffect, useId, useRef, useState, type FormEvent } from 'react'
import { Alert, Badge, Button, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toUnits } from '@/features/accounting/money'
import { formatDate, today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import type { ReceivedTaxInvoice } from './types'

/** ข้อมูลที่ส่งเมื่อบันทึกใบกำกับภาษีที่ได้รับ — POST {apiPath} */
export function taxInvoicePayload(v: { no: string; date: string; vat: string }, suggested: string): Record<string, string> {
  const body: Record<string, string> = { tax_invoice_no: v.no.trim(), tax_invoice_date: v.date }
  const vat = normalizeAmount(v.vat)
  if (vat && vat !== normalizeAmount(suggested)) body.vat = vat
  return body
}

/**
 * ใบกำกับภาษีที่ได้รับของซื้อบริการ — รายการ + ช่องบันทึก (ตั้งต้นภาษีตามสัดส่วนที่จ่ายแล้ว / ที่รอทั้งหมด จาก API) + ยกเลิกรายใบ
 * บันทึก = Dr ภาษีซื้อ / Cr ภาษีซื้อยังไม่ถึงกำหนด (ลงบัญชีที่ backend)
 */
export function TaxInvoicesPanel({
  apiPath,
  records,
  withInvoice,
  undue,
  suggested,
  canPost,
  entryBase,
}: {
  apiPath: string
  records: ReceivedTaxInvoice[]
  withInvoice: { no: string; date: string; vat: string } | null
  undue: string
  suggested: string
  canPost: boolean
  entryBase: string | null
}) {
  const v = texts.purchasing.vat
  const router = useRouter()
  const id = useId()
  const headingRef = useRef<HTMLHeadingElement>(null)
  const formRef = useRef<HTMLFormElement>(null)
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState({ no: '', date: today(), vat: suggested })
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const pending = toUnits(undue) > 0n

  useEffect(() => {
    if (open) headingRef.current?.focus()
  }, [open])

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: Record<string, string> = {}
    if (!form.no.trim()) errs.no = requiredMessage(v.fields.no)
    if (!form.date) errs.date = requiredMessage(v.fields.date)
    const amount = normalizeAmount(form.vat)
    if (amount === null || amount === '' || toUnits(amount) <= 0n) errs.vat = v.invalid
    else if (toUnits(amount) > toUnits(undue)) errs.vat = format(v.overUndue, { max: formatAmount(undue) })
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    setBusy(true)
    setError(null)
    try {
      await apiRequest(apiPath, { method: 'POST', body: taxInvoicePayload(form, suggested) })
      showToast(format(v.recorded, { no: form.no.trim() }))
      setOpen(false)
      setForm({ no: '', date: today(), vat: '' })
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  async function cancel(r: ReceivedTaxInvoice) {
    if (!window.confirm(format(v.confirmCancel, { no: r.tax_invoice_no }))) return
    setError(null)
    try {
      await apiRequest(`${apiPath}/${r.id}`, { method: 'PATCH', body: { status: 'cancelled' } })
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  const th = 'px-3 py-2.5 text-left text-[13px] font-medium text-ink-muted'
  const num = 'px-3 py-2.5 text-right tabular-nums whitespace-nowrap'
  return (
    <section aria-labelledby={`${id}-h`} className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 id={`${id}-h`} className="text-[15px] font-semibold">
          {v.taxInvoices}
        </h2>
        {canPost && pending && !open && (
          <Button
            type="button"
            onClick={() => {
              setForm({ no: '', date: today(), vat: suggested })
              setOpen(true)
            }}
          >
            <FileCheck2 aria-hidden />
            {v.record}
          </Button>
        )}
      </div>
      {error && <Alert tone="error">{error}</Alert>}

      {open && (
        <form
          ref={formRef}
          noValidate
          onSubmit={submit}
          onKeyDown={(e) => e.key === 'Escape' && setOpen(false)}
          aria-labelledby={`${id}-f`}
          className="flex flex-col gap-4 rounded-(--radius-panel) border border-primary/40 bg-surface p-5 shadow-card"
        >
          <div className="flex items-start justify-between gap-3">
            <h3 id={`${id}-f`} ref={headingRef} tabIndex={-1} className="text-[15px] font-semibold outline-none">
              {v.record}
            </h3>
            <button
              type="button"
              aria-label={texts.common.cancel}
              onClick={() => setOpen(false)}
              className="inline-flex size-9 cursor-pointer items-center justify-center rounded-(--radius-control) text-ink-muted hover:bg-subtle"
            >
              <X aria-hidden className="size-4" />
            </button>
          </div>
          <div className="grid gap-4 sm:grid-cols-3">
            <TextField label={v.fields.no} required value={form.no} maxLength={50} error={errors.no} onChange={(e) => setForm((f) => ({ ...f, no: e.target.value }))} />
            <TextField label={v.fields.date} required type="date" value={form.date} error={errors.date} onChange={(e) => setForm((f) => ({ ...f, date: e.target.value }))} />
            <TextField
              label={v.fields.vat}
              required
              inputMode="decimal"
              value={form.vat}
              error={errors.vat}
              className="[&_input]:text-right [&_input]:tabular-nums"
              onChange={(e) => setForm((f) => ({ ...f, vat: e.target.value }))}
            />
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="secondary" onClick={() => setOpen(false)}>
              {texts.common.cancel}
            </Button>
            <Button type="submit" loading={busy} loadingText={texts.common.saving}>
              {v.record}
            </Button>
          </div>
        </form>
      )}

      <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
        <table className="w-full min-w-[40rem] border-collapse text-sm">
          <thead>
            <tr className="border-b border-line bg-subtle">
              <th scope="col" className={th}>
                {v.fields.no}
              </th>
              <th scope="col" className={th}>
                {v.fields.date}
              </th>
              <th scope="col" className={`${th} text-right`}>
                {v.fields.base}
              </th>
              <th scope="col" className={`${th} text-right`}>
                {v.fields.vat}
              </th>
              <th scope="col" className={th}>
                {v.fields.entry}
              </th>
              <th scope="col" className={th}>
                <span className="sr-only">{v.fields.status}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {withInvoice && (
              <tr className="border-b border-line last:border-b-0">
                <td className="px-3 py-2.5 font-mono text-[13px]">{withInvoice.no}</td>
                <td className="px-3 py-2.5 whitespace-nowrap">{formatDate(withInvoice.date)}</td>
                <td className={num}>–</td>
                <td className={num}>{formatAmount(withInvoice.vat)}</td>
                <td className="px-3 py-2.5">–</td>
                <td className="px-3 py-2.5" />
              </tr>
            )}
            {records.map((r) => (
              <tr key={r.id} className={`border-b border-line last:border-b-0 ${r.status === 'cancelled' ? 'text-ink-muted line-through' : ''}`}>
                <td className="px-3 py-2.5 font-mono text-[13px]">{r.tax_invoice_no}</td>
                <td className="px-3 py-2.5 whitespace-nowrap">{formatDate(r.tax_invoice_date)}</td>
                <td className={num}>{formatAmount(r.base)}</td>
                <td className={num}>{formatAmount(r.vat)}</td>
                <td className="px-3 py-2.5">
                  {entryBase && r.entry_id ? (
                    <Link href={`${entryBase}/${r.entry_id}`} className="text-primary hover:underline">
                      {r.entry_no}
                    </Link>
                  ) : (
                    r.entry_no || '–'
                  )}
                </td>
                <td className="px-3 py-2.5 text-right">
                  {r.status === 'cancelled' ? (
                    <Badge tone="warning">{texts.purchasing.statuses.posted.cancelled}</Badge>
                  ) : (
                    canPost && (
                      <Button type="button" variant="secondary" onClick={() => cancel(r)}>
                        {v.cancel}
                      </Button>
                    )
                  )}
                </td>
              </tr>
            ))}
            {!withInvoice && records.length === 0 && (
              <tr>
                <td colSpan={6} className="px-3 py-4 text-center text-ink-muted">
                  {v.none}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </section>
  )
}
