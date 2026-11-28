'use client'

import { CircleCheck, Lock } from 'lucide-react'
import { useRouter } from 'next/navigation'
import { useEffect, useId, useState, type FormEvent, type ReactNode } from 'react'
import { Alert, Button, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { businessTypeName, type BusinessType } from '@/config/businessTypes'
import { AddressFields, emptyAddress, FormBar, StatusSelect, type Address } from '@/features/forms/FormParts'
import { cx } from '@/lib/cx'
import { apiRequest, errorMessage } from '@/lib/http'
import { focusFirstInvalid, patterns, requiredMessage, validateAddress, type FieldErrors } from '@/lib/validation'

export type Company = {
  id: string
  code: string
  name: string
  tax_id: string
  legal_name_th: string
  legal_name_en: string
  address: Address
  phone: string
  email: string
  website: string
  base_currency: string
  fiscal_year_start_month: number
  amount_decimals: number
  quantity_decimals: number
  status: string
  /** กำหนดตอนสร้างบริษัท แก้ไม่ได้ (ไม่ส่งไปกับ PUT) */
  business_type?: BusinessType
}

type Field = 'name' | 'base_currency' | 'email' | 'phone' | 'postal_code' | 'country'
const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const CURRENCY = /^[A-Z]{3}$/
const SAVED_MS = 4000

/** ตัวอย่างตัวเลขตามจำนวนทศนิยม เช่น 2 → 1,234.57 */
function decimalsExample(n: number): string {
  return (1234.5678).toLocaleString('th-TH', { minimumFractionDigits: n, maximumFractionDigits: n })
}

/**
 * ข้อมูลบริษัท (PUT /companies/{id} — แทนทั้งก้อน ยกเว้นรหัส) — หน้าตั้งค่าแบบ 2 คอลัมน์ (หัวข้อ + คำอธิบาย | ช่องกรอก)
 *
 * - แถบบันทึกติดล่างจอ: บอกว่ามีการแก้ไขค้างอยู่, ปุ่มยกเลิกการแก้ไข / บันทึก (บันทึกได้เมื่อมีการแก้ไข)
 * - ยืนยันก่อนบันทึกสิ่งที่ย้อนกลับไม่ได้: ปิดใช้งานบริษัท, เพิ่มทศนิยม (ลดกลับไม่ได้)
 * - บันทึกสำเร็จ: ข้อความในแถบบันทึก (aria-live) หายเองใน 4 วินาที — อยู่หน้าเดิม
 */
export function CompanySettingsForm({ company, canManage }: { company: Company; canManage: boolean }) {
  const t = texts.company
  const f = t.fields
  const router = useRouter()
  const readOnly = !canManage
  const initial = { ...company, address: { ...emptyAddress, ...company.address } }
  const [base, setBase] = useState(initial)
  const [v, setV] = useState(initial)
  const [errors, setErrors] = useState<FieldErrors<Field>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)
  const [busy, setBusy] = useState(false)
  const dirty = JSON.stringify(v) !== JSON.stringify(base)

  useEffect(() => {
    if (!saved) return
    const id = window.setTimeout(() => setSaved(false), SAVED_MS)
    return () => window.clearTimeout(id)
  }, [saved])

  const set = <K extends keyof typeof v>(key: K, value: (typeof v)[K]) => {
    setSaved(false)
    setV((s) => ({ ...s, [key]: value }))
  }
  const decimals = (min: number) => Array.from({ length: 7 - min }, (_, i) => ({ value: String(min + i), label: String(min + i) }))
  const endMonth = t.months[(v.fiscal_year_start_month + 10) % 12]!

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (readOnly || !dirty) return
    const errs: FieldErrors<Field> = validateAddress(v.address)
    if (!v.name.trim()) errs.name = requiredMessage(f.name)
    if (!CURRENCY.test(v.base_currency)) errs.base_currency = texts.validation.currency
    if (v.email.trim() && !EMAIL.test(v.email.trim())) errs.email = texts.validation.email
    if (v.phone.trim() && !patterns.phone.test(v.phone.trim())) errs.phone = texts.validation.phone
    setErrors(errs)
    setFormError(null)
    setSaved(false)
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)

    // ยืนยันการเปลี่ยนที่ย้อนกลับไม่ได้ / มีผลกับทั้งบริษัท
    if (v.status === 'inactive' && base.status !== 'inactive' && !window.confirm(t.confirm.inactive)) return
    if ((v.amount_decimals > base.amount_decimals || v.quantity_decimals > base.quantity_decimals) && !window.confirm(t.confirm.decimals)) return

    setBusy(true)
    try {
      const { id: _id, code: _code, business_type: _type, ...body } = v
      const next = { ...v, name: v.name.trim(), address: { ...v.address, country: v.address.country.trim().toUpperCase() || 'TH' } }
      await apiRequest(`/companies/${company.id}`, { method: 'PUT', body: { ...body, name: next.name, address: next.address } })
      setBase(next)
      setV(next)
      setSaved(true)
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  function discard() {
    setV(base)
    setErrors({})
    setFormError(null)
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      {formError && <Alert tone="error">{formError}</Alert>}
      {readOnly && <Alert>{texts.common.readOnly}</Alert>}

      <SettingsSection title={t.sections.general}>
        <ReadOnlyValue label={f.code} value={v.code} />
        {v.business_type && <ReadOnlyValue label={texts.businessTypes.label} value={businessTypeName(v.business_type)} />}
        <TextField label={f.name} required value={v.name} maxLength={200} disabled={readOnly} error={errors.name} onChange={(e) => set('name', e.target.value)} />
        <TextField label={f.legalNameTh} value={v.legal_name_th} maxLength={300} disabled={readOnly} onChange={(e) => set('legal_name_th', e.target.value)} />
        <TextField label={f.legalNameEn} value={v.legal_name_en} maxLength={300} disabled={readOnly} onChange={(e) => set('legal_name_en', e.target.value)} />
        <TextField label={f.taxId} value={v.tax_id} maxLength={20} inputMode="numeric" disabled={readOnly} onChange={(e) => set('tax_id', e.target.value)} />
        <TextField label={f.phone} value={v.phone} maxLength={30} type="tel" autoComplete="tel" disabled={readOnly} error={errors.phone} onChange={(e) => set('phone', e.target.value)} />
        <TextField label={f.email} value={v.email} maxLength={320} type="email" autoComplete="email" disabled={readOnly} error={errors.email} onChange={(e) => set('email', e.target.value)} />
        <TextField label={f.website} value={v.website} maxLength={300} type="url" autoComplete="url" disabled={readOnly} onChange={(e) => set('website', e.target.value)} />
      </SettingsSection>

      <SettingsSection title={t.sections.address}>
        <AddressFields value={v.address} onChange={(a) => set('address', a)} disabled={readOnly} errors={errors} />
      </SettingsSection>

      <SettingsSection title={t.sections.accounting}>
        <TextField
          label={f.baseCurrency}
          required          value={v.base_currency}
          maxLength={3}
          disabled={readOnly}
          error={errors.base_currency}
          onChange={(e) => set('base_currency', e.target.value.toUpperCase())}
        />
        <div className="flex flex-col gap-1.5">
          <SelectField
            label={f.fiscalYearStartMonth}
            value={String(v.fiscal_year_start_month)}
            disabled={readOnly}
            options={t.months.map((m, i) => ({ value: String(i + 1), label: m }))}
            onChange={(e) => set('fiscal_year_start_month', Number(e.target.value))}
          />
          <p className="text-[13px] text-ink-muted">{format(t.fiscalRange, { start: t.months[v.fiscal_year_start_month - 1]!, end: endMonth })}</p>
        </div>
        <DecimalsField
          label={f.amountDecimals}
          value={v.amount_decimals}
          disabled={readOnly}
          options={decimals(company.amount_decimals)}
          onChange={(n) => set('amount_decimals', n)}
        />
        <DecimalsField
          label={f.quantityDecimals}
          value={v.quantity_decimals}
          disabled={readOnly}
          options={decimals(company.quantity_decimals)}
          onChange={(n) => set('quantity_decimals', n)}
        />
      </SettingsSection>

      <SettingsSection title={t.sections.status} tone={v.status === 'inactive' ? 'warning' : undefined}>
        <StatusSelect value={v.status} disabled={readOnly} onChange={(s) => set('status', s)} />
      </SettingsSection>

      {canManage && (
        <FormBar>
          <p aria-live="polite" className="flex min-h-5 items-center gap-1.5 text-[13px]">
            {saved ? (
              <span className="flex items-center gap-1.5 font-medium text-success">
                <CircleCheck aria-hidden className="size-4" />
                {t.savedAt}
              </span>
            ) : dirty ? (
              <span className="flex items-center gap-1.5 text-ink-muted">
                <span aria-hidden className="size-2 rounded-full bg-accent" />
                {t.dirty}
              </span>
            ) : null}
          </p>
          <div className="ml-auto flex gap-2">
            {dirty && (
              <Button variant="ghost" disabled={busy} onClick={discard}>
                {t.discard}
              </Button>
            )}
            <Button type="submit" loading={busy} loadingText={texts.common.saving} disabled={!dirty}>
              {texts.common.save}
            </Button>
          </div>
        </FormBar>
      )}
    </form>
  )
}

/** กลุ่มการตั้งค่า: ซ้าย = หัวข้อ, ขวา = ช่องกรอก (จอแคบเรียงบนลงล่าง) */
function SettingsSection({ title, tone, children }: { title: string; tone?: 'warning'; children: ReactNode }) {
  const id = useId()
  return (
    <section aria-labelledby={id} className="grid gap-4 border-b border-line pb-8 last-of-type:border-b-0 lg:grid-cols-[minmax(0,15rem)_minmax(0,1fr)] lg:gap-10">
      <div>
        <h2 id={id} className="text-[15px] font-semibold">
          {title}
        </h2>
      </div>
      <div
        className={cx(
          'grid gap-x-5 gap-y-4 rounded-(--radius-panel) border bg-surface p-5 shadow-card sm:grid-cols-2',
          tone === 'warning' ? 'border-warning/40 bg-warning-soft/40' : 'border-line',
        )}
      >
        {children}
      </div>
    </section>
  )
}

/** ค่าที่แก้ไม่ได้ — แสดงเป็นข้อความ (ไม่ใช่ช่องที่ถูกปิด) ตามหลัก read-only ≠ disabled */
function ReadOnlyValue({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-[13px] font-medium text-ink">{label}</span>
      <span className="flex h-10 items-center gap-2 rounded-(--radius-control) bg-neutral-soft px-3 text-sm font-medium">
        {value}
        <Lock aria-hidden className="ml-auto size-3.5 text-ink-subtle" />
      </span>
    </div>
  )
}

/** ทศนิยม: เลือกได้ตั้งแต่ค่าปัจจุบันขึ้นไป + ตัวอย่างตัวเลข */
function DecimalsField({
  label,
  value,
  disabled,
  options,
  onChange,
}: {
  label: string
  value: number
  disabled: boolean
  options: { value: string; label: string }[]
  onChange: (n: number) => void
}) {
  const t = texts.company
  return (
    <div className="flex flex-col gap-1.5">
      <SelectField label={label} value={String(value)} disabled={disabled} options={options} onChange={(e) => onChange(Number(e.target.value))} />
      <p className="flex flex-wrap items-center gap-2 text-[13px] text-ink-muted">
        <span className="tabular-nums">{format(t.decimalsExample, { value: decimalsExample(value) })}</span>
      </p>
    </div>
  )
}
