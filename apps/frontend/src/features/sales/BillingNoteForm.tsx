'use client'

import { useRouter } from 'next/navigation'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Alert, Badge, Button, ButtonLink, ReferenceSelect, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { FormBar, FormSection } from '@/features/forms/FormParts'
import { cx } from '@/lib/cx'
import { formatDate, today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import { addDays } from './SalesDocForm'
import type { BillingNoteInvoice, InvoiceSummary, Ref } from './types'

/**
 * ออกใบวางบิล — เลือกลูกค้าแล้วแสดงใบขายสินค้า / ใบแจ้งหนี้ที่ยังค้างชำระ (2 คำขอขนานกัน: /sales-invoices + /service-invoices)
 * ไม่มีผลทางบัญชี / ภาษี; ใบที่อยู่ในใบวางบิลอื่นที่ยังเปิด backend ปฏิเสธ (422)
 */
export function BillingNoteForm({
  apiPath,
  customersApi,
  invoicesApi,
  serviceInvoicesApi,
  docsPath,
  listHref,
}: {
  apiPath: string
  customersApi: string
  invoicesApi: string
  serviceInvoicesApi: string
  docsPath: string
  listHref: string
}) {
  const t = texts.sales
  const b = t.billingNotes
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const [customer, setCustomer] = useState<Ref | null>(null)
  const [head, setHead] = useState({ date: today(), due: addDays(today(), 7), description: '' })
  const [invoices, setInvoices] = useState<BillingNoteInvoice[] | null>(null)
  const [loading, setLoading] = useState(false)
  const [picked, setPicked] = useState<Record<string, boolean>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!customer) {
      setInvoices(null)
      return
    }
    let live = true
    setLoading(true)
    const q = `?customer_id=${encodeURIComponent(customer.id)}&status=posted&page_size=100`
    const load = (api: string, kind: BillingNoteInvoice['kind']) =>
      apiRequest<InvoiceSummary[]>(`${api}${q}`).then(
        (rows) =>
          rows
            .filter((r) => toUnits(r.outstanding) > 0n)
            .map((r) => ({ id: r.id, document_no: r.document_no, kind, date: r.date, due_date: r.due_date, gross: r.totals.gross, outstanding: r.outstanding })),
        () => [] as BillingNoteInvoice[],
      )
    Promise.all([load(invoicesApi, 'invoice'), load(serviceInvoicesApi, 'service_invoice')])
      .then(([a, s]) => {
        if (!live) return
        setInvoices([...a, ...s].sort((x, y) => x.due_date.localeCompare(y.due_date)))
        setPicked({})
      })
      .finally(() => live && setLoading(false))
    return () => {
      live = false
    }
  }, [customer, invoicesApi, serviceInvoicesApi])

  const chosen = (invoices ?? []).filter((i) => picked[i.id])
  const total = chosen.reduce((s, i) => s + toUnits(i.outstanding), 0n)
  const allOn = !!invoices?.length && invoices.every((i) => picked[i.id])

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setFormError(null)
    const errs: Record<string, string> = {}
    if (!customer) errs.customer = requiredMessage(t.fields.customer)
    if (!head.date) errs.date = requiredMessage(t.fields.date)
    if (!head.due) errs.due = requiredMessage(b.fields.dueDate)
    if (customer && chosen.length === 0) errs.invoices = b.noneSelected
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    setBusy(true)
    try {
      const saved = await apiRequest<{ id: string; document_no: string }>(apiPath, {
        method: 'POST',
        body: { customer_id: customer!.id, date: head.date, due_date: head.due, description: head.description.trim(), invoice_ids: chosen.map((i) => i.id) },
      })
      showToast(format(t.toast.posted, { no: saved.document_no }))
      router.push(`${docsPath}/${saved.id}`)
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
      setBusy(false)
    }
  }

  return (
    <form ref={formRef} noValidate onSubmit={submit} className="flex flex-col gap-6">
      {formError && <Alert tone="error">{formError}</Alert>}
      <FormSection title={t.sections.header}>
        <ReferenceSelect
          label={t.fields.customer}
          required
          source={customersApi}
          params={{ status: 'active' }}
          value={customer}
          onChange={setCustomer}
          error={errors.customer}
          texts={texts.reference}
          className="sm:col-span-2"
        />
        <TextField label={t.fields.date} required type="date" value={head.date} error={errors.date} onChange={(e) => setHead((h) => ({ ...h, date: e.target.value }))} />
        <TextField label={b.fields.dueDate} required type="date" value={head.due} error={errors.due} onChange={(e) => setHead((h) => ({ ...h, due: e.target.value }))} />
        <TextField label={t.fields.description} value={head.description} maxLength={500} className="sm:col-span-2" onChange={(e) => setHead((h) => ({ ...h, description: e.target.value }))} />
      </FormSection>

      <section aria-labelledby="bn-invoices" className="flex min-w-0 flex-col gap-3">
        <h2 id="bn-invoices" className="text-[15px] font-semibold">
          {b.fields.invoices}
        </h2>
        <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
          {!customer || loading || !invoices?.length ? (
            <p className="px-4 py-10 text-center text-sm text-ink-muted" aria-live="polite">
              {!customer ? b.pickCustomer : loading ? b.loading : b.none}
            </p>
          ) : (
            <table className="w-full min-w-[40rem] border-collapse text-sm">
              <thead>
                <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
                  <th scope="col" className="w-12 px-3 py-2.5">
                    <input
                      type="checkbox"
                      aria-label={texts.finance.invoices.selectAll}
                      checked={allOn}
                      onChange={(e) => setPicked(Object.fromEntries(invoices.map((i) => [i.id, e.target.checked])))}
                      className="size-4 cursor-pointer accent-primary"
                    />
                  </th>
                  <th scope="col" className="px-2 py-2.5 font-medium">
                    {t.columns.no}
                  </th>
                  <th scope="col" className="px-2 py-2.5 font-medium">
                    {t.columns.due}
                  </th>
                  <th scope="col" className="px-2 py-2.5 text-right font-medium">
                    {t.columns.total}
                  </th>
                  <th scope="col" className="px-2 py-2.5 text-right font-medium">
                    {t.columns.outstanding}
                  </th>
                </tr>
              </thead>
              <tbody>
                {invoices.map((i) => (
                  <tr key={i.id} className={cx('border-b border-line last:border-b-0', picked[i.id] && 'bg-primary-soft/30')}>
                    <td className="px-3 py-2">
                      <input
                        type="checkbox"
                        aria-label={format(b.select, { no: i.document_no })}
                        checked={!!picked[i.id]}
                        onChange={(e) => setPicked((p) => ({ ...p, [i.id]: e.target.checked }))}
                        className="size-4 cursor-pointer accent-primary"
                      />
                    </td>
                    <td className="px-2 py-2">
                      <span className="font-mono text-[13px]">{i.document_no}</span>
                      <span className="ml-2">
                        <Badge tone={i.kind === 'service_invoice' ? 'info' : 'neutral'}>{t.kinds[i.kind] ?? i.kind}</Badge>
                      </span>
                    </td>
                    <td className="px-2 py-2 whitespace-nowrap">{formatDate(i.due_date)}</td>
                    <td className="px-2 py-2 text-right tabular-nums">{formatAmount(i.gross)}</td>
                    <td className="px-2 py-2 text-right font-medium tabular-nums">{formatAmount(i.outstanding)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
        {errors.invoices && (
          <p role="alert" className="text-[13px] text-danger">
            {errors.invoices}
          </p>
        )}
        <p aria-live="polite" className="text-right text-sm">
          <span className="text-ink-muted">{b.fields.total}</span> <span className="ml-2 font-semibold tabular-nums">{formatAmount(total)}</span>
        </p>
      </section>

      <FormBar>
        <div className="ml-auto flex gap-2">
          <ButtonLink href={listHref} variant="secondary">
            {texts.common.cancel}
          </ButtonLink>
          <Button type="submit" loading={busy} loadingText={texts.common.saving}>
            {t.actions.billingNote}
          </Button>
        </div>
      </FormBar>
    </form>
  )
}
