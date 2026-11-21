'use client'

import { useState, type FormEvent } from 'react'
import { Alert, Badge, TextField } from '@/components'
import { format, texts } from '@/config'
import { AddressFields, emptyAddress, FormActions, FormNotices, FormSection, StatusSelect } from '@/features/forms/FormParts'
import { useResourceForm } from '@/features/forms/useResourceForm'
import { focusFirstInvalid, normalizeCode, patterns, requiredMessage, validateAddress, validateCodeName, type FieldErrors } from '@/lib/validation'
import { branchesPath, type Branch, type BranchInput } from './types'

type Field = 'code' | 'name' | 'tax_branch_no' | 'phone' | 'postal_code' | 'country'

export type BranchFormProps = {
  companyId: string
  /** ไม่มี = สร้างใหม่ */
  branch?: Branch
  /** มีสิทธิ์ organization.branch:manage — ไม่มี = ดูอย่างเดียว */
  canManage: boolean
  /** หน้ารายการ (รวมเงื่อนไขค้นหาเดิม) — กลับไปที่นี่หลังบันทึก / ลบ / ยกเลิก */
  listHref: string
}

/**
 * ฟอร์มสาขา: สร้าง (POST) / แก้ไข (PUT แทนทั้งก้อน) / ลบ (DELETE)
 * ตรวจรูปแบบก่อนส่งเพื่อช่วยผู้ใช้ — กติกาจริงอยู่ที่ backend (เช่น สำนักงานใหญ่ปิดใช้งาน / ลบไม่ได้)
 */
export function BranchForm({ companyId, branch, canManage, listHref }: BranchFormProps) {
  const t = texts.branches.form
  const headOffice = branch?.is_head_office ?? false
  const readOnly = !canManage
  const form = useResourceForm({ apiPath: branchesPath(companyId), id: branch?.id, listHref })

  const [values, setValues] = useState<BranchInput>(() =>
    branch
      ? { code: branch.code, name: branch.name, tax_branch_no: branch.tax_branch_no, phone: branch.phone, status: branch.status, address: { ...emptyAddress, ...branch.address } }
      : { code: '', name: '', tax_branch_no: '', phone: '', status: 'active', address: emptyAddress },
  )
  const [errors, setErrors] = useState<FieldErrors<Field>>({})
  const set = <K extends keyof BranchInput>(key: K, value: BranchInput[K]) => setValues((v) => ({ ...v, [key]: value }))

  function validate(): FieldErrors<Field> {
    const v = values
    const e: FieldErrors<Field> = { ...validateCodeName(v, t), ...validateAddress(v.address) }
    if (!v.tax_branch_no) e.tax_branch_no = requiredMessage(t.taxBranchNo)
    else if (!patterns.taxBranchNo.test(v.tax_branch_no)) e.tax_branch_no = texts.validation.taxBranchNo
    if (v.phone.trim() && !patterns.phone.test(v.phone.trim())) e.phone = texts.validation.phone
    return e
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (readOnly) return
    const e = validate()
    setErrors(e)
    if (Object.keys(e).length > 0) return focusFirstInvalid(event.currentTarget)
    form.save({
      ...values,
      name: values.name.trim(),
      phone: values.phone.trim(),
      address: { ...values.address, country: values.address.country.trim().toUpperCase() || 'TH' },
    } satisfies BranchInput)
  }

  return (
    <form noValidate onSubmit={handleSubmit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly}>
        {headOffice && <Alert>{texts.branches.headOfficeHint}</Alert>}
      </FormNotices>

      <FormSection title={t.general} description={texts.branches.headOfficeHint} aside={headOffice ? <Badge tone="info">{texts.branches.headOffice}</Badge> : undefined}>
        <TextField
          label={t.code}
          required
          hint={t.codeHint}
          value={values.code}
          maxLength={20}
          autoComplete="off"
          disabled={readOnly}
          error={errors.code}
          onChange={(e) => set('code', normalizeCode(e.target.value))}
        />
        <TextField label={t.name} required value={values.name} maxLength={200} disabled={readOnly} error={errors.name} onChange={(e) => set('name', e.target.value)} />
        <TextField
          label={t.taxBranchNo}
          required
          hint={t.taxBranchNoHint}
          value={values.tax_branch_no}
          maxLength={5}
          inputMode="numeric"
          disabled={readOnly || headOffice}
          error={errors.tax_branch_no}
          onChange={(e) => set('tax_branch_no', e.target.value.replace(/\D/g, ''))}
        />
        <TextField label={t.phone} value={values.phone} maxLength={30} type="tel" disabled={readOnly} error={errors.phone} onChange={(e) => set('phone', e.target.value)} />
        {branch && (
          <StatusSelect value={values.status} disabled={readOnly || headOffice} onChange={(s) => set('status', s as BranchInput['status'])} />
        )}
      </FormSection>

      <FormSection title={t.address} description={texts.common.sections.addressHint}>
        <AddressFields value={values.address} onChange={(a) => set('address', a)} disabled={readOnly} errors={errors} />
      </FormSection>

      <FormActions
        listHref={listHref}
        canManage={canManage}
        canDelete={!!branch && !headOffice}
        busy={form.busy}
        onDelete={() => form.remove(format(texts.branches.deleteConfirm, { code: branch?.code ?? '' }))}
      />
    </form>
  )
}
