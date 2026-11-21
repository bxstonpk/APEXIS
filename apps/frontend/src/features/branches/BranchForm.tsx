'use client'

import { useRouter } from 'next/navigation'
import { useState, type FormEvent, type ReactNode } from 'react'
import { Alert, Badge, Button, ButtonLink, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { apiRequest, errorMessage } from '@/lib/http'
import { normalizeCode, patterns, requiredMessage, type FieldErrors } from '@/lib/validation'
import { branchesPath, emptyAddress, type Branch, type BranchInput } from './types'

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
  const router = useRouter()
  const t = texts.branches.form
  const isNew = !branch
  const headOffice = branch?.is_head_office ?? false
  const readOnly = !canManage

  const [values, setValues] = useState<BranchInput>(() =>
    branch
      ? { code: branch.code, name: branch.name, tax_branch_no: branch.tax_branch_no, phone: branch.phone, status: branch.status, address: { ...emptyAddress, ...branch.address } }
      : { code: '', name: '', tax_branch_no: '', phone: '', status: 'active', address: emptyAddress },
  )
  const [errors, setErrors] = useState<FieldErrors<Field>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState<'save' | 'delete' | null>(null)

  const set = <K extends keyof BranchInput>(key: K, value: BranchInput[K]) => setValues((v) => ({ ...v, [key]: value }))
  const setAddress = (key: keyof BranchInput['address'], value: string) =>
    setValues((v) => ({ ...v, address: { ...v.address, [key]: value } }))

  function validate(): FieldErrors<Field> {
    const e: FieldErrors<Field> = {}
    const v = values
    if (!v.code) e.code = requiredMessage(t.code)
    else if (!patterns.code.test(v.code)) e.code = texts.validation.code
    if (!v.name.trim()) e.name = requiredMessage(t.name)
    if (!v.tax_branch_no) e.tax_branch_no = requiredMessage(t.taxBranchNo)
    else if (!patterns.taxBranchNo.test(v.tax_branch_no)) e.tax_branch_no = texts.validation.taxBranchNo
    if (v.phone.trim() && !patterns.phone.test(v.phone.trim())) e.phone = texts.validation.phone
    const country = v.address.country.trim().toUpperCase() || 'TH'
    if (!patterns.country.test(country)) e.country = texts.validation.country
    else if (country === 'TH' && v.address.postal_code.trim() && !patterns.thPostalCode.test(v.address.postal_code.trim())) {
      e.postal_code = texts.validation.postalCode
    }
    return e
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (readOnly) return
    const e = validate()
    setErrors(e)
    setFormError(null)
    if (Object.keys(e).length > 0) return
    setBusy('save')
    const body: BranchInput = {
      ...values,
      name: values.name.trim(),
      phone: values.phone.trim(),
      address: { ...values.address, country: values.address.country.trim().toUpperCase() || 'TH' },
    }
    try {
      if (isNew) await apiRequest(branchesPath(companyId), { method: 'POST', body })
      else await apiRequest(`${branchesPath(companyId)}/${branch.id}`, { method: 'PUT', body })
      router.push(listHref)
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
      setBusy(null)
    }
  }

  async function handleDelete() {
    if (!branch || !window.confirm(format(texts.branches.deleteConfirm, { code: branch.code }))) return
    setFormError(null)
    setBusy('delete')
    try {
      await apiRequest(`${branchesPath(companyId)}/${branch.id}`, { method: 'DELETE' })
      router.push(listHref)
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
      setBusy(null)
    }
  }

  return (
    <form noValidate onSubmit={handleSubmit} className="flex flex-col gap-6">
      {formError && <Alert tone="error">{formError}</Alert>}
      {readOnly && <Alert>{texts.common.readOnly}</Alert>}
      {headOffice && <Alert>{texts.branches.headOfficeHint}</Alert>}

      <Section title={t.general} aside={headOffice ? <Badge tone="info">{texts.branches.headOffice}</Badge> : undefined}>
        <TextField
          label={t.code}
          hint={t.codeHint}
          value={values.code}
          maxLength={20}
          autoComplete="off"
          disabled={readOnly}
          error={errors.code}
          onChange={(e) => set('code', normalizeCode(e.target.value))}
        />
        <TextField label={t.name} value={values.name} maxLength={200} disabled={readOnly} error={errors.name} onChange={(e) => set('name', e.target.value)} />
        <TextField
          label={t.taxBranchNo}
          hint={t.taxBranchNoHint}
          value={values.tax_branch_no}
          maxLength={5}
          inputMode="numeric"
          disabled={readOnly || headOffice}
          error={errors.tax_branch_no}
          onChange={(e) => set('tax_branch_no', e.target.value.replace(/\D/g, ''))}
        />
        <TextField label={t.phone} value={values.phone} maxLength={30} type="tel" disabled={readOnly} error={errors.phone} onChange={(e) => set('phone', e.target.value)} />
        {!isNew && (
          <SelectField
            label={t.status}
            value={values.status}
            disabled={readOnly || headOffice}
            options={[
              { value: 'active', label: texts.common.statuses.active! },
              { value: 'inactive', label: texts.common.statuses.inactive! },
            ]}
            onChange={(e) => set('status', e.target.value as BranchInput['status'])}
          />
        )}
      </Section>

      <Section title={t.address}>
        <TextField label={t.addressLine} value={values.address.line} maxLength={300} disabled={readOnly} className="sm:col-span-2" onChange={(e) => setAddress('line', e.target.value)} />
        <TextField label={t.subDistrict} value={values.address.sub_district} maxLength={100} disabled={readOnly} onChange={(e) => setAddress('sub_district', e.target.value)} />
        <TextField label={t.district} value={values.address.district} maxLength={100} disabled={readOnly} onChange={(e) => setAddress('district', e.target.value)} />
        <TextField label={t.province} value={values.address.province} maxLength={100} disabled={readOnly} onChange={(e) => setAddress('province', e.target.value)} />
        <TextField
          label={t.postalCode}
          value={values.address.postal_code}
          maxLength={20}
          inputMode="numeric"
          disabled={readOnly}
          error={errors.postal_code}
          onChange={(e) => setAddress('postal_code', e.target.value)}
        />
        <TextField
          label={t.country}
          value={values.address.country}
          maxLength={2}
          disabled={readOnly}
          error={errors.country}
          onChange={(e) => setAddress('country', e.target.value.toUpperCase())}
        />
      </Section>

      <div className="flex flex-wrap items-center gap-2">
        {!isNew && canManage && !headOffice && (
          <Button variant="danger" loading={busy === 'delete'} loadingText={texts.common.deleting} disabled={busy !== null} onClick={handleDelete}>
            {texts.common.delete}
          </Button>
        )}
        <div className="ml-auto flex gap-2">
          <ButtonLink href={listHref} variant="secondary">
            {readOnly ? texts.common.back : texts.common.cancel}
          </ButtonLink>
          {canManage && (
            <Button type="submit" loading={busy === 'save'} loadingText={texts.common.saving} disabled={busy !== null}>
              {texts.common.save}
            </Button>
          )}
        </div>
      </div>
    </form>
  )
}

function Section({ title, aside, children }: { title: string; aside?: ReactNode; children: ReactNode }) {
  return (
    <fieldset className="rounded-xl border border-line bg-surface p-5">
      <legend className="flex items-center gap-2 px-1 font-semibold">
        {title}
        {aside}
      </legend>
      <div className="grid gap-4 sm:grid-cols-2">{children}</div>
    </fieldset>
  )
}
