'use client'

import { useRouter } from 'next/navigation'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Alert, Button, ButtonLink, ReferenceSelect, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toUnits } from '@/features/accounting/money'
import { FormBar, FormSection } from '@/features/forms/FormParts'
import { today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import { lineAmounts } from './calc'
import type { Vat } from './SalesDocForm'
import type { Order, Ref } from './types'

/**
 * ใบรับเงินมัดจำ / ใบกำกับภาษี — ภาษีขายเกิดวันที่เอกสาร (รับเงินก่อนส่งมอบ / ให้บริการ)
 * บันทึกเป็นลูกหนี้ แล้วยืนยันรับเงินที่แอปการเงิน; หักในเอกสารขายได้เมื่อรับเงินครบแล้ว
 * ?order_id= = มัดจำของใบสั่งขาย (ลูกค้า / สาขา / เงื่อนไข VAT ตามใบสั่งขาย)
 */
export function DepositForm({
  apiPath,
  customersApi,
  docsPath,
  listHref,
  vat,
  order,
}: {
  apiPath: string
  customersApi: string
  docsPath: string
  listHref: string
  vat: Vat
  order?: Order
}) {
  const t = texts.sales
  const f = t.deposits.fields
  const router = useRouter()
  const formRef = useRef<HTMLFormElement>(null)
  const [customer, setCustomer] = useState<Ref | null>(order?.customer ?? null)
  const [branchNo, setBranchNo] = useState(order?.customer_branch_no ?? '')
  const [branchOptions, setBranchOptions] = useState<{ value: string; label: string }[]>([])
  const [head, setHead] = useState({ date: today(), description: '', amount: '', inclusive: order?.price_includes_vat ?? false })
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!customer || order) {
      setBranchOptions([])
      return
    }
    let live = true
    apiRequest<{ branch_no: string; branches?: { branch_no: string; name: string }[] }>(`${customersApi}/${customer.id}`)
      .then((c) => {
        if (!live) return
        const main = c.branch_no === '00000' ? t.print.headOffice : texts.partners.branches.main
        const opts = [{ value: c.branch_no, label: `${c.branch_no} ${main}` }, ...(c.branches ?? []).map((b) => ({ value: b.branch_no, label: `${b.branch_no} ${b.name}`.trim() }))]
        setBranchOptions(opts)
        setBranchNo((cur) => (opts.some((o) => o.value === cur) ? cur : c.branch_no))
      })
      .catch(() => live && setBranchOptions([]))
    return () => {
      live = false
    }
  }, [customer, order, customersApi, t.print.headOffice])

  const rate = order ? order.vat_rate : vat?.registered ? vat.rate : '0'
  const known = !!order || vat !== null
  const amounts = lineAmounts('1', normalizeAmount(head.amount) ?? '0', known ? rate : '0', head.inclusive)

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setFormError(null)
    const errs: Record<string, string> = {}
    if (!customer) errs.customer = requiredMessage(t.fields.customer)
    if (!head.date) errs.date = requiredMessage(t.fields.date)
    if (!head.description.trim()) errs.description = t.deposits.descriptionRequired
    const n = normalizeAmount(head.amount)
    if (!n || toUnits(n) <= 0n) errs.amount = t.deposits.amountInvalid
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(formRef.current)
    const body: Record<string, unknown> = {
      customer_id: customer!.id,
      customer_branch_no: branchNo,
      order_id: order?.id ?? null,
      date: head.date,
      description: head.description.trim(),
      amount: n,
      price_includes_vat: head.inclusive,
    }
    setBusy(true)
    try {
      const saved = await apiRequest<{ id: string; document_no: string }>(apiPath, { method: 'POST', body })
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
        {order ? (
          <TextField label={t.fields.customer} value={`${order.customer.code} ${order.customer.name}`} readOnly className="sm:col-span-2" />
        ) : (
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
        )}
        {branchOptions.length > 1 && (
          <SelectField label={t.fields.customerBranch} value={branchNo} options={branchOptions} onChange={(e) => setBranchNo(e.target.value)} className="sm:col-span-2" />
        )}
        <TextField label={t.fields.date} required type="date" value={head.date} error={errors.date} onChange={(e) => setHead((h) => ({ ...h, date: e.target.value }))} />
        <TextField
          label={f.amount}
          required
          inputMode="decimal"          value={head.amount}
          error={errors.amount}
          className="[&_input]:text-right [&_input]:tabular-nums"
          onChange={(e) => setHead((h) => ({ ...h, amount: e.target.value }))}
        />
        <TextField
          label={f.description}
          required
          value={head.description}
          maxLength={500}
          error={errors.description}
          className="sm:col-span-2"
          onChange={(e) => setHead((h) => ({ ...h, description: e.target.value }))}
        />
        {!order && (
          <label className="flex cursor-pointer items-start gap-3 sm:col-span-2">
            <input
              type="checkbox"
              checked={head.inclusive}
              disabled={vat?.registered === false}
              onChange={(e) => setHead((h) => ({ ...h, inclusive: e.target.checked }))}
              className="mt-0.5 size-4 accent-primary"
            />
            <span className="block text-sm font-medium">{t.fields.priceIncludesVat}</span>
          </label>
        )}
      </FormSection>

      <div className="flex justify-end">
        <dl aria-live="polite" className="w-full max-w-sm rounded-(--radius-panel) border border-line bg-surface p-4 text-sm shadow-card">
          <div className="flex justify-between gap-4">
            <dt className="text-ink-muted">{f.base}</dt>
            <dd className="tabular-nums">{formatAmount(amounts.net)}</dd>
          </div>
          <div className="mt-2 flex justify-between gap-4">
            <dt className="text-ink-muted">{format(t.totals.vat, { rate: known ? rate : '–' })}</dt>
            <dd className="tabular-nums">{known ? formatAmount(amounts.vat) : '–'}</dd>
          </div>
          <div className="mt-2 flex justify-between gap-4 border-t border-line pt-2 font-semibold">
            <dt>{f.gross}</dt>
            <dd className="tabular-nums">{formatAmount(amounts.gross)}</dd>
          </div>
        </dl>
      </div>

      <FormBar>
        <div className="ml-auto flex gap-2">
          <ButtonLink href={listHref} variant="secondary">
            {texts.common.cancel}
          </ButtonLink>
          <Button type="submit" loading={busy} loadingText={texts.common.saving}>
            {t.actions.postSale}
          </Button>
        </div>
      </FormBar>
    </form>
  )
}
