'use client'

import { useState, type FormEvent } from 'react'
import { Button, ReferenceSelect, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { normalizeAmount, toUnits } from '@/features/accounting/money'
import { FormActions, FormBar, FormNotices, FormSection, StatusSelect } from '@/features/forms/FormParts'
import { orNull, useResourceForm } from '@/features/forms/useResourceForm'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, normalizeCode, requiredMessage, type FieldErrors } from '@/lib/validation'
import type { Customer, Ref, SalesSettings } from './types'

// ---------------------------------------------------------------- ลูกค้า

export function CustomerForm({ apiPath, listHref, canManage, item }: { apiPath: string; listHref: string; canManage: boolean; item?: Customer }) {
  const t = texts.sales.customers
  const f = t.fields
  const readOnly = !canManage
  const form = useResourceForm({ apiPath, id: item?.id, listHref })
  const [v, setV] = useState({
    code: item?.code ?? '',
    name: item?.name ?? '',
    tax_id: item?.tax_id ?? '',
    branch_no: item?.branch_no ?? '00000',
    address: item?.address ?? '',
    payment_term_days: String(item?.payment_term_days ?? 30),
    tax_type: item?.tax_type ?? 'juristic',
    credit_limit: item?.credit_limit ?? '',
    status: item?.status ?? 'active',
  })
  const [errors, setErrors] = useState<FieldErrors<'code' | 'name' | 'tax_id' | 'branch_no' | 'term' | 'credit'>>({})
  const set = (k: keyof typeof v, val: string) => setV((s) => ({ ...s, [k]: val }))

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: typeof errors = {}
    if (!v.code) errs.code = requiredMessage(f.code)
    if (!v.name.trim()) errs.name = requiredMessage(f.name)
    if (v.tax_id && !/^\d{13}$/.test(v.tax_id)) errs.tax_id = texts.validation.taxId
    if (v.branch_no && !/^\d{5}$/.test(v.branch_no)) errs.branch_no = texts.validation.taxBranchNo
    const term = Number(v.payment_term_days)
    if (!/^\d+$/.test(v.payment_term_days) || term > 365) errs.term = requiredMessage(f.term)
    const credit = v.credit_limit.trim() ? normalizeAmount(v.credit_limit) : null
    if (v.credit_limit.trim() && (credit === null || toUnits(credit) < 0n)) errs.credit = t.creditHint
    setErrors(errs)
    if (readOnly) return
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    form.save({
      code: v.code,
      name: v.name.trim(),
      tax_id: v.tax_id,
      branch_no: v.branch_no,
      address: v.address.trim(),
      payment_term_days: term,
      tax_type: v.tax_type,
      credit_limit: credit,
      status: v.status,
    })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly} />
      <FormSection title={texts.branches.form.general} description={texts.common.sections.generalHint}>
        <TextField label={f.code} required value={v.code} maxLength={30} disabled={readOnly} error={errors.code} onChange={(e) => set('code', normalizeCode(e.target.value))} />
        <TextField label={f.name} required value={v.name} maxLength={200} disabled={readOnly} error={errors.name} onChange={(e) => set('name', e.target.value)} />
        {item && <StatusSelect value={v.status} disabled={readOnly} onChange={(s) => set('status', s)} />}
      </FormSection>
      <FormSection title={t.sections.tax} description={t.sections.taxHint}>
        <SelectField
          label={f.taxType}
          value={v.tax_type}
          disabled={readOnly}
          options={Object.entries(t.taxTypes).map(([value, label]) => ({ value, label }))}
          onChange={(e) => set('tax_type', e.target.value)}
        />
        <TextField
          label={f.taxId}
          value={v.tax_id}
          maxLength={13}
          inputMode="numeric"
          disabled={readOnly}
          error={errors.tax_id}
          onChange={(e) => set('tax_id', e.target.value.replace(/\D/g, ''))}
        />
        <TextField
          label={f.branchNo}
          hint={t.branchHint}
          value={v.branch_no}
          maxLength={5}
          inputMode="numeric"
          disabled={readOnly}
          error={errors.branch_no}
          onChange={(e) => set('branch_no', e.target.value.replace(/\D/g, ''))}
        />
        <label className="flex flex-col gap-1.5 sm:col-span-2">
          <span className="text-sm font-medium">{f.address}</span>
          <textarea
            value={v.address}
            maxLength={500}
            rows={3}
            disabled={readOnly}
            onChange={(e) => set('address', e.target.value)}
            className="rounded-(--radius-control) border border-line-strong bg-surface px-3 py-2 text-base outline-none focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-primary/30 disabled:bg-subtle disabled:text-ink-muted sm:text-sm"
          />
        </label>
      </FormSection>
      <FormSection title={t.sections.credit}>
        <TextField
          label={f.term}
          inputMode="numeric"
          value={v.payment_term_days}
          disabled={readOnly}
          error={errors.term}
          onChange={(e) => set('payment_term_days', e.target.value.replace(/\D/g, ''))}
        />
        <TextField
          label={f.creditLimit}
          hint={t.creditHint}
          inputMode="decimal"
          value={v.credit_limit}
          disabled={readOnly}
          error={errors.credit}
          className="[&_input]:text-right [&_input]:tabular-nums"
          onChange={(e) => set('credit_limit', e.target.value)}
        />
      </FormSection>
      <FormActions
        listHref={listHref}
        canManage={canManage}
        canDelete={!!item}
        busy={form.busy}
        onDelete={() => form.remove(format(t.deleteConfirm, { code: item?.code ?? '' }))}
      />
    </form>
  )
}

// ---------------------------------------------------------------- ค่าตั้งการขาย

type AccountKey = 'journal_id' | 'ar_account_id' | 'output_vat_account_id' | 'revenue_account_id' | 'cogs_account_id'

export function SalesSettingsForm({
  apiPath,
  journalsApi,
  accountsApi,
  canManage,
  settings,
  refs,
}: {
  apiPath: string
  journalsApi: string
  accountsApi: string
  canManage: boolean
  settings: SalesSettings
  refs: Record<AccountKey, Ref | null>
}) {
  const t = texts.sales.settings
  const f = t.fields
  const readOnly = !canManage
  const [vat, setVat] = useState({ registered: settings.vat_registered, rate: settings.vat_rate })
  const [v, setV] = useState(refs)
  const [rateError, setRateError] = useState<string>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const set = (k: AccountKey) => (r: Ref | null) => setV((s) => ({ ...s, [k]: r }))

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (readOnly) return
    const rate = normalizeAmount(vat.rate)
    if (vat.registered && (rate === null || toUnits(rate) <= 0n || toUnits(rate) >= toUnits('100'))) {
      setRateError(requiredMessage(f.vatRate))
      return focusFirstInvalid(e.currentTarget)
    }
    setRateError(undefined)
    setBusy(true)
    setError(null)
    try {
      const body = {
        vat_registered: vat.registered,
        vat_rate: vat.registered ? rate : (normalizeAmount(vat.rate) ?? settings.vat_rate),
        ...Object.fromEntries(Object.entries(v).map(([k, r]) => [k, orNull(r?.id)])),
      }
      await apiRequest(apiPath, { method: 'PUT', body })
      showToast(texts.common.toast.saved)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const account = (k: AccountKey, label: string, type: string) => (
    <ReferenceSelect label={label} source={accountsApi} params={{ postable: 'true', status: 'active', type }} value={v[k]} onChange={set(k)} disabled={readOnly} texts={texts.reference} />
  )

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={error} readOnly={readOnly} />
      <FormSection title={t.vat} description={t.vatHint}>
        <label className="flex cursor-pointer items-center gap-3 sm:col-span-2">
          <input
            type="checkbox"
            checked={vat.registered}
            disabled={readOnly}
            onChange={(e) => setVat((s) => ({ ...s, registered: e.target.checked }))}
            className="size-4 accent-primary"
          />
          <span className="text-sm font-medium">{f.vatRegistered}</span>
        </label>
        {vat.registered && (
          <TextField
            label={f.vatRate}
            required
            inputMode="decimal"
            value={vat.rate}
            disabled={readOnly}
            error={rateError}
            onChange={(e) => setVat((s) => ({ ...s, rate: e.target.value }))}
          />
        )}
      </FormSection>
      <FormSection title={t.accounts} description={t.accountsHint}>
        <ReferenceSelect label={f.journal} source={journalsApi} params={{ status: 'active' }} value={v.journal_id} onChange={set('journal_id')} disabled={readOnly} texts={texts.reference} />
        {account('ar_account_id', f.ar, 'asset')}
        {account('output_vat_account_id', f.outputVat, 'liability')}
        {account('revenue_account_id', f.revenue, 'revenue')}
        {account('cogs_account_id', f.cogs, 'expense')}
      </FormSection>
      {canManage && (
        <FormBar>
          <div className="ml-auto">
            <Button type="submit" loading={busy} loadingText={texts.common.saving}>
              {texts.common.save}
            </Button>
          </div>
        </FormBar>
      )}
    </form>
  )
}
