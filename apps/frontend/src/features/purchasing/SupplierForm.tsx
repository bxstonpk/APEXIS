'use client'

import { useState, type FormEvent } from 'react'
import { SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { FormActions, FormNotices, FormSection, StatusSelect } from '@/features/forms/FormParts'
import { useResourceForm } from '@/features/forms/useResourceForm'
import { BranchesEditor, branchErrors, branchesFrom, branchesPayload, newBranch, type BranchDraft } from '@/features/partners/BranchesEditor'
import { cleanAddress, type ThaiAddress, type VatRegistrant } from '@/features/partners/thaiAddress'
import { ThaiAddressFields } from '@/features/partners/ThaiAddressFields'
import { isTaxId, VatLookup } from '@/features/partners/VatLookup'
import { focusFirstInvalid, normalizeCode, requiredMessage, type FieldErrors } from '@/lib/validation'
import type { Supplier } from './types'

/**
 * ผู้ขาย — เริ่มจากเลขผู้เสียภาษี: พบในฐานข้อมูลผู้จด VAT = เติมชื่อ / สาขา / ที่อยู่ และเลือก "จด VAT" ให้
 * ไม่พบ = กรอกเอง (สถานะ VAT ตั้งเอง)
 */
export function SupplierForm({
  apiPath,
  lookupApi,
  suppliersPath,
  listHref,
  canManage,
  item,
}: {
  apiPath: string
  lookupApi: string
  /** path หน้าผู้ขาย (ลิงก์ไปรหัสเดิมเมื่อเลขผู้เสียภาษีซ้ำ) */
  suppliersPath: string
  listHref: string
  canManage: boolean
  item?: Supplier
}) {
  const t = texts.purchasing.suppliers
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
    status: item?.status ?? 'active',
  })
  const [vat, setVat] = useState(item?.vat_registered ?? false)
  const [address, setAddress] = useState<ThaiAddress>(() => cleanAddress(item?.address_detail))
  const [branches, setBranches] = useState<BranchDraft[]>(() => branchesFrom(item?.branches))
  const [branchErrs, setBranchErrs] = useState<Record<number, string>>({})
  const [errors, setErrors] = useState<FieldErrors<'code' | 'name' | 'tax_id' | 'branch_no' | 'term' | 'postal'>>({})
  const set = (k: keyof typeof v, val: string) => setV((s) => ({ ...s, [k]: val }))

  function apply(r: VatRegistrant) {
    setV((s) => ({ ...s, name: r.name, branch_no: r.branch_no, tax_type: r.tax_type }))
    setVat(true)
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
      vat_registered: vat,
      tax_type: v.tax_type,
      status: v.status,
      branches: branchesPayload(branches),
    })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly} />
      <FormSection title={t.sections.tax}>
        <VatLookup
          source={lookupApi}
          taxId={v.tax_id}
          onTaxIdChange={(id) => set('tax_id', id)}
          onApply={apply}
          onAddBranch={addBranch}
          addedBranches={[v.branch_no, ...branches.map((b) => b.branch_no)]}
          owner={{ source: apiPath, path: suppliersPath, selfId: item?.id }}
          error={errors.tax_id}
          disabled={readOnly}
          autoFetch={!item}
        />
        <TextField
          label={f.branchNo}          value={v.branch_no}
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
        <label className="flex cursor-pointer items-start gap-3 sm:col-span-2">
          <input type="checkbox" checked={vat} disabled={readOnly} onChange={(e) => setVat(e.target.checked)} className="mt-0.5 size-4 accent-primary" />
          <span className="block text-sm font-medium">{f.vatRegistered}</span>
        </label>
      </FormSection>
      <FormSection title={texts.branches.form.general}>
        <TextField label={f.code} required value={v.code} maxLength={30} disabled={readOnly} error={errors.code} onChange={(e) => set('code', normalizeCode(e.target.value))} />
        <TextField label={f.name} required value={v.name} maxLength={200} disabled={readOnly} error={errors.name} onChange={(e) => set('name', e.target.value)} />
        {item && <StatusSelect value={v.status} disabled={readOnly} onChange={(s) => set('status', s)} />}
      </FormSection>
      <FormSection title={texts.partners.address.section}>
        <ThaiAddressFields value={address} onChange={setAddress} disabled={readOnly} postalError={errors.postal} />
      </FormSection>
      <FormSection title={texts.partners.branches.section}>
        <BranchesEditor value={branches} onChange={setBranches} errors={branchErrs} disabled={readOnly} />
      </FormSection>
      <FormSection title={t.sections.terms}>
        <TextField
          label={f.term}
          inputMode="numeric"
          value={v.payment_term_days}
          disabled={readOnly}
          error={errors.term}
          onChange={(e) => set('payment_term_days', e.target.value.replace(/\D/g, ''))}
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
