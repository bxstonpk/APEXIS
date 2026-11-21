'use client'

import { Trash2 } from 'lucide-react'
import { useId, type ReactNode } from 'react'
import { Alert, Button, ButtonLink, SelectField, TextField } from '@/components'
import { texts } from '@/config'
import type { FieldErrors } from '@/lib/validation'

/**
 * กลุ่มช่องในฟอร์ม — แบบเดียวกับหน้าข้อมูลบริษัท:
 * จอกว้าง = ซ้ายหัวข้อ + คำอธิบาย, ขวากรอบช่องกรอก (2 คอลัมน์); จอแคบเรียงบนลงล่าง
 */
export function FormSection({ title, description, aside, children }: { title: string; description?: string; aside?: ReactNode; children: ReactNode }) {
  const id = useId()
  return (
    <section aria-labelledby={id} className="grid gap-4 border-b border-line pb-8 last-of-type:border-b-0 lg:grid-cols-[minmax(0,15rem)_minmax(0,1fr)] lg:gap-10">
      <div>
        <h2 id={id} className="flex flex-wrap items-center gap-2 text-[15px] font-semibold">
          {title}
          {aside}
        </h2>
        {description && <p className="mt-1 text-[13px] leading-relaxed text-ink-muted">{description}</p>}
      </div>
      <div className="grid gap-x-5 gap-y-4 rounded-(--radius-panel) border border-line bg-surface p-5 shadow-card sm:grid-cols-2">{children}</div>
    </section>
  )
}

/** ข้อความด้านบนฟอร์ม: error จาก API, อ่านอย่างเดียว */
export function FormNotices({ formError, readOnly, children }: { formError: string | null; readOnly: boolean; children?: ReactNode }) {
  return (
    <>
      {formError && <Alert tone="error">{formError}</Alert>}
      {readOnly && <Alert>{texts.common.readOnly}</Alert>}
      {children}
    </>
  )
}

/** แถบปุ่มท้ายฟอร์ม (ติดขอบล่างจอเมื่อฟอร์มยาว) */
export function FormBar({ children }: { children: ReactNode }) {
  return (
    <div className="sticky bottom-0 z-10 -mx-4 flex flex-wrap items-center gap-2 border-t border-line bg-surface/95 px-4 py-3 backdrop-blur sm:mx-0 sm:rounded-(--radius-panel) sm:border sm:shadow-card">
      {children}
    </div>
  )
}

/** ปุ่มท้ายฟอร์ม: [ลบ] ......... [ยกเลิก / กลับ] [บันทึก] */
export function FormActions({
  listHref,
  canManage,
  canDelete,
  busy,
  onDelete,
}: {
  listHref: string
  canManage: boolean
  canDelete: boolean
  busy: 'save' | 'delete' | null
  onDelete?: () => void
}) {
  return (
    <FormBar>
      {canManage && canDelete && onDelete && (
        <Button variant="danger" loading={busy === 'delete'} loadingText={texts.common.deleting} disabled={busy !== null} onClick={onDelete}>
          <Trash2 aria-hidden />
          {texts.common.delete}
        </Button>
      )}
      <div className="ml-auto flex gap-2">
        <ButtonLink href={listHref} variant="secondary">
          {canManage ? texts.common.cancel : texts.common.back}
        </ButtonLink>
        {canManage && (
          <Button type="submit" loading={busy === 'save'} loadingText={texts.common.saving} disabled={busy !== null}>
            {texts.common.save}
          </Button>
        )}
      </div>
    </FormBar>
  )
}

/** ตัวเลือกสถานะ active / inactive จาก config */
export function StatusSelect({ value, onChange, disabled }: { value: string; onChange: (v: string) => void; disabled?: boolean }) {
  return (
    <SelectField
      label={texts.common.status}
      value={value}
      disabled={disabled}
      options={[
        { value: 'active', label: texts.common.statuses.active! },
        { value: 'inactive', label: texts.common.statuses.inactive! },
      ]}
      onChange={(e) => onChange(e.target.value)}
    />
  )
}

export type Address = {
  line: string
  sub_district: string
  district: string
  province: string
  postal_code: string
  country: string
}

export const emptyAddress: Address = { line: '', sub_district: '', district: '', province: '', postal_code: '', country: 'TH' }

/** ช่องที่อยู่แบบไทย (ใช้ร่วม: สาขา, โครงการ, บริษัท) */
export function AddressFields({
  value,
  onChange,
  disabled,
  errors = {},
}: {
  value: Address
  onChange: (next: Address) => void
  disabled?: boolean
  errors?: FieldErrors<'postal_code' | 'country'>
}) {
  const t = texts.branches.form
  const set = (key: keyof Address, v: string) => onChange({ ...value, [key]: v })
  return (
    <>
      <TextField label={t.addressLine} value={value.line} maxLength={300} disabled={disabled} className="sm:col-span-2" onChange={(e) => set('line', e.target.value)} />
      <TextField label={t.subDistrict} value={value.sub_district} maxLength={100} disabled={disabled} onChange={(e) => set('sub_district', e.target.value)} />
      <TextField label={t.district} value={value.district} maxLength={100} disabled={disabled} onChange={(e) => set('district', e.target.value)} />
      <TextField label={t.province} value={value.province} maxLength={100} disabled={disabled} onChange={(e) => set('province', e.target.value)} />
      <TextField
        label={t.postalCode}
        value={value.postal_code}
        maxLength={20}
        inputMode="numeric"
        disabled={disabled}
        error={errors.postal_code}
        onChange={(e) => set('postal_code', e.target.value)}
      />
      <TextField
        label={t.country}
        value={value.country}
        maxLength={2}
        disabled={disabled}
        error={errors.country}
        onChange={(e) => set('country', e.target.value.toUpperCase())}
      />
    </>
  )
}
