'use client'

import { useState, type FormEvent } from 'react'
import { ReferenceSelect, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { AddressFields, emptyAddress, FormActions, FormNotices, FormSection, StatusSelect, type Address } from '@/features/forms/FormParts'
import { orNull, useResourceForm } from '@/features/forms/useResourceForm'
import {
  isDateRangeValid,
  focusFirstInvalid,
  normalizeCode,
  requiredMessage,
  validateAddress,
  validateCodeName,
  type FieldErrors,
} from '@/lib/validation'
import type { BusinessUnit, CostCenter, Department, ProjectSite, Ref } from './resources'

/**
 * ฟอร์มข้อมูลหลักของแอปองค์กร (สร้าง / แก้ไข / ลบ) — โครงเดียวกับฟอร์มสาขา
 * ตรวจรูปแบบก่อนส่งเพื่อช่วยผู้ใช้ — กติกาจริงอยู่ที่ backend (เช่น ลำดับชั้นวน, ถูกอ้างอิงอยู่)
 */

type CommonProps = {
  companyId: string
  canManage: boolean
  listHref: string
  /** API ของรายการ เช่น /companies/{id}/departments */
  apiPath: string
}

const refTexts = texts.reference

function CodeNameFields({
  values,
  set,
  errors,
  labels,
  readOnly,
}: {
  values: { code: string; name: string }
  set: (key: 'code' | 'name', v: string) => void
  errors: FieldErrors<'code' | 'name'>
  labels: { code: string; name: string }
  readOnly: boolean
}) {
  return (
    <>
      <TextField
        label={labels.code}
        required
        hint={texts.branches.form.codeHint}
        value={values.code}
        maxLength={20}
        autoComplete="off"
        disabled={readOnly}
        error={errors.code}
        onChange={(e) => set('code', normalizeCode(e.target.value))}
      />
      <TextField label={labels.name} required value={values.name} maxLength={200} disabled={readOnly} error={errors.name} onChange={(e) => set('name', e.target.value)} />
    </>
  )
}

// ---------------------------------------------------------------- หน่วยธุรกิจ

export function BusinessUnitForm({ canManage, listHref, apiPath, item }: CommonProps & { item?: BusinessUnit }) {
  const t = texts.businessUnits.fields
  const readOnly = !canManage
  const form = useResourceForm({ apiPath, id: item?.id, listHref })
  const [v, setV] = useState({ code: item?.code ?? '', name: item?.name ?? '', status: item?.status ?? 'active' })
  const [parent, setParent] = useState<Ref | null>(item?.parent ?? null)
  const [errors, setErrors] = useState<FieldErrors<'code' | 'name'>>({})

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs = validateCodeName(v, t)
    setErrors(errs)
    if (readOnly) return
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    form.save({ code: v.code, name: v.name.trim(), parent_id: parent?.id ?? null, status: v.status })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly} />
      <FormSection title={texts.branches.form.general} description={texts.common.sections.generalHint}>
        <CodeNameFields values={v} set={(k, val) => setV((s) => ({ ...s, [k]: val }))} errors={errors} labels={t} readOnly={readOnly} />
        {item && <StatusSelect value={v.status} disabled={readOnly} onChange={(s) => setV((x) => ({ ...x, status: s }))} />}
      </FormSection>
      <FormSection title={texts.common.sections.relations} description={texts.common.sections.relationsHint}>
        <ReferenceSelect
          label={t.parent}
          source={apiPath}
          params={{ status: 'active' }}
          value={parent}
          onChange={setParent}
          excludeId={item?.id}
          disabled={readOnly}
          texts={refTexts}
        />
      </FormSection>
      <FormActions
        listHref={listHref}
        canManage={canManage}
        canDelete={!!item}
        busy={form.busy}
        onDelete={() => form.remove(format(texts.businessUnits.deleteConfirm, { code: item?.code ?? '' }))}
      />
    </form>
  )
}

// ---------------------------------------------------------------- แผนก

export function DepartmentForm({ companyId, canManage, listHref, apiPath, item }: CommonProps & { item?: Department }) {
  const t = texts.departments.fields
  const readOnly = !canManage
  const form = useResourceForm({ apiPath, id: item?.id, listHref })
  const [v, setV] = useState({ code: item?.code ?? '', name: item?.name ?? '', status: item?.status ?? 'active' })
  const [parent, setParent] = useState<Ref | null>(item?.parent ?? null)
  const [unit, setUnit] = useState<Ref | null>(item?.business_unit ?? null)
  const [branch, setBranch] = useState<Ref | null>(item?.branch ?? null)
  const [errors, setErrors] = useState<FieldErrors<'code' | 'name'>>({})
  const base = `/companies/${companyId}`

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs = validateCodeName(v, t)
    setErrors(errs)
    if (readOnly) return
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    form.save({
      code: v.code,
      name: v.name.trim(),
      parent_id: parent?.id ?? null,
      business_unit_id: unit?.id ?? null,
      branch_id: branch?.id ?? null,
      status: v.status,
    })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly} />
      <FormSection title={texts.branches.form.general} description={texts.common.sections.generalHint}>
        <CodeNameFields values={v} set={(k, val) => setV((s) => ({ ...s, [k]: val }))} errors={errors} labels={t} readOnly={readOnly} />
        {item && <StatusSelect value={v.status} disabled={readOnly} onChange={(s) => setV((x) => ({ ...x, status: s }))} />}
      </FormSection>
      <FormSection title={texts.common.sections.relations} description={texts.common.sections.relationsHint}>
        <ReferenceSelect label={t.parent} source={apiPath} params={{ status: 'active' }} value={parent} onChange={setParent} excludeId={item?.id} disabled={readOnly} texts={refTexts} />
        <ReferenceSelect label={t.businessUnit} source={`${base}/business-units`} params={{ status: 'active' }} value={unit} onChange={setUnit} disabled={readOnly} texts={refTexts} />
        <ReferenceSelect label={t.branch} source={`${base}/branches`} params={{ status: 'active' }} value={branch} onChange={setBranch} disabled={readOnly} texts={refTexts} />
      </FormSection>
      <FormActions
        listHref={listHref}
        canManage={canManage}
        canDelete={!!item}
        busy={form.busy}
        onDelete={() => form.remove(format(texts.departments.deleteConfirm, { code: item?.code ?? '' }))}
      />
    </form>
  )
}

// ---------------------------------------------------------------- ศูนย์ต้นทุน

export function CostCenterForm({ companyId, canManage, listHref, apiPath, item, defaultValidFrom }: CommonProps & { item?: CostCenter; defaultValidFrom: string }) {
  const t = texts.costCenters.fields
  const readOnly = !canManage
  const form = useResourceForm({ apiPath, id: item?.id, listHref })
  const [v, setV] = useState({
    code: item?.code ?? '',
    name: item?.name ?? '',
    status: item?.status ?? 'active',
    valid_from: item?.valid_from ?? defaultValidFrom,
    valid_to: item?.valid_to ?? '',
  })
  const [department, setDepartment] = useState<Ref | null>(item?.department ?? null)
  const [errors, setErrors] = useState<FieldErrors<'code' | 'name' | 'valid_from' | 'valid_to'>>({})

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: typeof errors = validateCodeName(v, t)
    if (!v.valid_from) errs.valid_from = requiredMessage(t.validFrom)
    else if (!isDateRangeValid(v.valid_from, v.valid_to)) errs.valid_to = texts.validation.dateRange
    setErrors(errs)
    if (readOnly) return
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    form.save({
      code: v.code,
      name: v.name.trim(),
      department_id: department?.id ?? null,
      valid_from: v.valid_from,
      valid_to: orNull(v.valid_to),
      status: v.status,
    })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly} />
      <FormSection title={texts.branches.form.general} description={texts.common.sections.generalHint}>
        <CodeNameFields values={v} set={(k, val) => setV((s) => ({ ...s, [k]: val }))} errors={errors} labels={t} readOnly={readOnly} />
        {item && <StatusSelect value={v.status} disabled={readOnly} onChange={(s) => setV((x) => ({ ...x, status: s }))} />}
      </FormSection>
      <FormSection title={texts.common.sections.relations} description={texts.common.sections.relationsHint}>
        <ReferenceSelect
          label={t.department}
          source={`/companies/${companyId}/departments`}
          params={{ status: 'active' }}
          value={department}
          onChange={setDepartment}
          disabled={readOnly}
          texts={refTexts}
          className="sm:col-span-2"
        />
      </FormSection>
      <FormSection title={texts.common.sections.period} description={texts.common.sections.periodHint}>
        <TextField label={t.validFrom} required type="date" value={v.valid_from} disabled={readOnly} error={errors.valid_from} onChange={(e) => setV((s) => ({ ...s, valid_from: e.target.value }))} />
        <TextField
          label={t.validTo}
          hint={t.validToHint}
          type="date"
          value={v.valid_to}
          min={v.valid_from || undefined}
          disabled={readOnly}
          error={errors.valid_to}
          onChange={(e) => setV((s) => ({ ...s, valid_to: e.target.value }))}
        />
      </FormSection>
      <FormActions
        listHref={listHref}
        canManage={canManage}
        canDelete={!!item}
        busy={form.busy}
        onDelete={() => form.remove(format(texts.costCenters.deleteConfirm, { code: item?.code ?? '' }))}
      />
    </form>
  )
}

// ---------------------------------------------------------------- โครงการ / ไซต์งาน

export function ProjectSiteForm({ companyId, canManage, listHref, apiPath, item, defaultStartDate }: CommonProps & { item?: ProjectSite; defaultStartDate: string }) {
  const t = texts.projectSites.fields
  const readOnly = !canManage
  const form = useResourceForm({ apiPath, id: item?.id, listHref })
  const [v, setV] = useState({
    code: item?.code ?? '',
    name: item?.name ?? '',
    status: item?.status ?? 'planned',
    start_date: item?.start_date ?? defaultStartDate,
    end_date: item?.end_date ?? '',
  })
  const [address, setAddress] = useState<Address>({ ...emptyAddress, ...item?.address })
  const [branch, setBranch] = useState<Ref | null>(item?.branch ?? null)
  const [costCenter, setCostCenter] = useState<Ref | null>(item?.cost_center ?? null)
  const [errors, setErrors] = useState<FieldErrors<'code' | 'name' | 'start_date' | 'end_date' | 'postal_code' | 'country'>>({})
  const base = `/companies/${companyId}`

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: typeof errors = { ...validateCodeName(v, t), ...validateAddress(address) }
    if (!v.start_date) errs.start_date = requiredMessage(t.startDate)
    else if (!isDateRangeValid(v.start_date, v.end_date)) errs.end_date = texts.validation.dateRange
    if (v.status === 'closed' && !v.end_date) errs.end_date = texts.validation.closedEndDate
    setErrors(errs)
    if (readOnly) return
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    form.save({
      code: v.code,
      name: v.name.trim(),
      branch_id: branch?.id ?? null,
      cost_center_id: costCenter?.id ?? null,
      address: { ...address, country: address.country.trim().toUpperCase() || 'TH' },
      start_date: v.start_date,
      end_date: orNull(v.end_date),
      status: v.status,
    })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly} />
      <FormSection title={texts.branches.form.general} description={texts.common.sections.generalHint}>
        <CodeNameFields values={v} set={(k, val) => setV((s) => ({ ...s, [k]: val }))} errors={errors} labels={t} readOnly={readOnly} />
        <SelectField
          label={t.status}
          value={v.status}
          disabled={readOnly}
          options={Object.entries(texts.projectSites.statuses).map(([value, label]) => ({ value, label }))}
          onChange={(e) => setV((s) => ({ ...s, status: e.target.value }))}
        />
      </FormSection>
      <FormSection title={texts.common.sections.relations} description={texts.common.sections.relationsHint}>
        <ReferenceSelect label={t.branch} source={`${base}/branches`} params={{ status: 'active' }} value={branch} onChange={setBranch} disabled={readOnly} texts={refTexts} />
        <ReferenceSelect label={t.costCenter} source={`${base}/cost-centers`} params={{ status: 'active' }} value={costCenter} onChange={setCostCenter} disabled={readOnly} texts={refTexts} />
      </FormSection>
      <FormSection title={texts.common.sections.period} description={texts.common.sections.periodHint}>
        <TextField label={t.startDate} required type="date" value={v.start_date} disabled={readOnly} error={errors.start_date} onChange={(e) => setV((s) => ({ ...s, start_date: e.target.value }))} />
        <TextField
          label={t.endDate}
          hint={t.endDateHint}
          type="date"
          value={v.end_date}
          min={v.start_date || undefined}
          disabled={readOnly}
          error={errors.end_date}
          onChange={(e) => setV((s) => ({ ...s, end_date: e.target.value }))}
        />
      </FormSection>
      <FormSection title={t.address} description={texts.common.sections.addressHint}>
        <AddressFields value={address} onChange={setAddress} disabled={readOnly} errors={errors} />
      </FormSection>
      <FormActions
        listHref={listHref}
        canManage={canManage}
        canDelete={!!item}
        busy={form.busy}
        onDelete={() => form.remove(format(texts.projectSites.deleteConfirm, { code: item?.code ?? '' }))}
      />
    </form>
  )
}
