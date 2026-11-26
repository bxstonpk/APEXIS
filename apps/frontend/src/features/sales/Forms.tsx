'use client'

import { useState, type FormEvent } from 'react'
import { Button, ReferenceSelect, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { normalizeAmount, toUnits } from '@/features/accounting/money'
import { FormActions, FormBar, FormNotices, FormSection, StatusSelect } from '@/features/forms/FormParts'
import { orNull, useResourceForm } from '@/features/forms/useResourceForm'
import { BranchesEditor, branchErrors, branchesFrom, branchesPayload, newBranch, type BranchDraft } from '@/features/partners/BranchesEditor'
import { cleanAddress, type ThaiAddress, type VatRegistrant } from '@/features/partners/thaiAddress'
import { ThaiAddressFields } from '@/features/partners/ThaiAddressFields'
import { isTaxId, VatLookup } from '@/features/partners/VatLookup'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, normalizeCode, requiredMessage, type FieldErrors } from '@/lib/validation'
import type { Customer, Ref, SalesSettings } from './types'

// ---------------------------------------------------------------- ลูกค้า

/**
 * ลูกค้า — เริ่มจากเลขผู้เสียภาษี: ผู้จด VAT ดึงชื่อ / สาขา / ที่อยู่จากกรมสรรพากร (ฟอร์มใหม่ดึงเองเมื่อครบ 13 หลัก)
 * ไม่พบ = กรอกเอง; ที่อยู่เก็บแยกช่องตามแบบสรรพากร
 */
export function CustomerForm({
  apiPath,
  lookupApi,
  customersPath,
  listHref,
  canManage,
  item,
}: {
  apiPath: string
  lookupApi: string
  /** path หน้าลูกค้า (ลิงก์ไปรหัสเดิมเมื่อเลขผู้เสียภาษีซ้ำ) */
  customersPath: string
  listHref: string
  canManage: boolean
  item?: Customer
}) {
  const t = texts.sales.customers
  const f = t.fields
  const readOnly = !canManage
  const form = useResourceForm({ apiPath, id: item?.id, listHref })
  const [v, setV] = useState({
    code: item?.code ?? '',
    name: item?.name ?? '',
    tax_id: item?.tax_id ?? '',
    branch_no: item?.branch_no ?? '00000',
    payment_term_days: String(item?.payment_term_days ?? 30),
    tax_type: item?.tax_type ?? 'juristic',
    credit_limit: item?.credit_limit ?? '',
    status: item?.status ?? 'active',
  })
  const [address, setAddress] = useState<ThaiAddress>(() => cleanAddress(item?.address_detail))
  const [branches, setBranches] = useState<BranchDraft[]>(() => branchesFrom(item?.branches))
  const [branchErrs, setBranchErrs] = useState<Record<number, string>>({})
  const [errors, setErrors] = useState<FieldErrors<'code' | 'name' | 'tax_id' | 'branch_no' | 'term' | 'credit' | 'postal'>>({})
  const set = (k: keyof typeof v, val: string) => setV((s) => ({ ...s, [k]: val }))

  function apply(r: VatRegistrant) {
    setV((s) => ({ ...s, name: r.name, branch_no: r.branch_no, tax_type: r.tax_type }))
    setAddress(cleanAddress(r.address_detail))
    setBranches((bs) => bs.filter((b) => b.branch_no !== r.branch_no))
    setErrors({})
  }

  function addBranch(r: VatRegistrant) {
    setBranches((bs) => [...bs, newBranch({ branch_no: r.branch_no, name: r.branch_name, address: cleanAddress(r.address_detail) })])
  }

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: typeof errors = {}
    if (!v.code) errs.code = requiredMessage(f.code)
    if (!v.name.trim()) errs.name = requiredMessage(f.name)
    if (v.tax_id && !isTaxId(v.tax_id)) errs.tax_id = texts.validation.taxId
    if (v.branch_no && !/^\d{5}$/.test(v.branch_no)) errs.branch_no = texts.validation.taxBranchNo
    const addr = cleanAddress(address)
    if (addr.postal_code && !/^\d{5}$/.test(addr.postal_code)) errs.postal = texts.validation.postalCode
    const term = Number(v.payment_term_days)
    if (!/^\d+$/.test(v.payment_term_days) || term > 365) errs.term = requiredMessage(f.term)
    const credit = v.credit_limit.trim() ? normalizeAmount(v.credit_limit) : null
    if (v.credit_limit.trim() && (credit === null || toUnits(credit) < 0n)) errs.credit = t.creditHint
    const bErrs = branchErrors(branches, v.branch_no)
    setErrors(errs)
    setBranchErrs(bErrs)
    if (readOnly) return
    if (Object.keys(errs).length || Object.keys(bErrs).length) return focusFirstInvalid(e.currentTarget)
    form.save({
      code: v.code,
      name: v.name.trim(),
      tax_id: v.tax_id,
      branch_no: v.branch_no,
      address_detail: addr,
      payment_term_days: term,
      tax_type: v.tax_type,
      credit_limit: credit,
      status: v.status,
      branches: branchesPayload(branches),
    })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly} />
      <FormSection title={t.sections.tax} description={t.sections.taxHint}>
        <VatLookup
          source={lookupApi}
          taxId={v.tax_id}
          onTaxIdChange={(id) => set('tax_id', id)}
          onApply={apply}
          onAddBranch={addBranch}
          addedBranches={[v.branch_no, ...branches.map((b) => b.branch_no)]}
          owner={{ source: apiPath, path: customersPath, selfId: item?.id }}
          error={errors.tax_id}
          disabled={readOnly}
          autoFetch={!item}
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
        <SelectField
          label={f.taxType}
          value={v.tax_type}
          disabled={readOnly}
          options={Object.entries(t.taxTypes).map(([value, label]) => ({ value, label }))}
          onChange={(e) => set('tax_type', e.target.value)}
        />
      </FormSection>
      <FormSection title={texts.branches.form.general} description={texts.common.sections.generalHint}>
        <TextField label={f.code} required value={v.code} maxLength={30} disabled={readOnly} error={errors.code} onChange={(e) => set('code', normalizeCode(e.target.value))} />
        <TextField label={f.name} required value={v.name} maxLength={200} disabled={readOnly} error={errors.name} onChange={(e) => set('name', e.target.value)} />
        {item && <StatusSelect value={v.status} disabled={readOnly} onChange={(s) => set('status', s)} />}
      </FormSection>
      <FormSection title={texts.partners.address.section} description={texts.partners.address.sectionHint}>
        <ThaiAddressFields value={address} onChange={setAddress} disabled={readOnly} postalError={errors.postal} />
      </FormSection>
      <FormSection title={texts.partners.branches.section} description={texts.partners.branches.sectionHint}>
        <BranchesEditor value={branches} onChange={setBranches} errors={branchErrs} disabled={readOnly} />
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

type AccountKey = 'journal_id' | 'ar_account_id' | 'output_vat_account_id' | 'revenue_account_id' | 'cogs_account_id' | 'sales_returns_account_id'

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
        {account('sales_returns_account_id', f.salesReturns, 'revenue')}
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
