'use client'

import { useRouter } from 'next/navigation'
import { useState, type FormEvent } from 'react'
import { Button, ButtonLink, TextField } from '@/components'
import { texts } from '@/config'
import { FormNotices, FormSection } from '@/features/forms/FormParts'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage, type FieldErrors } from '@/lib/validation'
import type { FiscalYear } from './resources'

/**
 * สร้างปีบัญชี (POST /fiscal-years) — backend สร้าง 12 งวดให้ใน transaction เดียว
 * สร้างเสร็จไปหน้าปีบัญชีนั้น ({listHref}/{id} — เห็นงวดทั้ง 12)
 */
export function FiscalYearForm({ apiPath, listHref, defaultYear }: { apiPath: string; listHref: string; defaultYear: number }) {
  const t = texts.fiscalYears.fields
  const router = useRouter()
  const [year, setYear] = useState(String(defaultYear))
  const [startDate, setStartDate] = useState(`${defaultYear}-01-01`)
  const [startTouched, setStartTouched] = useState(false)
  const [errors, setErrors] = useState<FieldErrors<'year' | 'start_date'>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  function changeYear(v: string) {
    setYear(v)
    // ยังไม่ได้แก้วันเริ่มเอง: เลื่อนปีของวันเริ่มตาม (คงเดือนเดิม)
    if (!startTouched && /^\d{4}$/.test(v)) setStartDate(`${v}${startDate.slice(4) || '-01-01'}`)
  }

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: FieldErrors<'year' | 'start_date'> = {}
    const y = Number(year)
    if (!year) errs.year = requiredMessage(t.year)
    else if (!Number.isInteger(y) || y < 1900 || y > 9999) errs.year = texts.validation.year
    if (!startDate) errs.start_date = requiredMessage(t.startDate)
    else if (!startDate.endsWith('-01')) errs.start_date = texts.validation.firstOfMonth
    setErrors(errs)
    setFormError(null)
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    setBusy(true)
    try {
      const fy = await apiRequest<FiscalYear>(apiPath, { method: 'POST', body: { year: y, start_date: startDate } })
      showToast(texts.common.toast.created)
      router.push(`${listHref}/${fy.id}`)
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
      setBusy(false)
    }
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={formError} readOnly={false} />
      <FormSection title={texts.fiscalYears.heading} description={texts.fiscalYears.description}>
        <TextField label={t.year} required hint={t.yearHint} inputMode="numeric" maxLength={4} value={year} error={errors.year} onChange={(e) => changeYear(e.target.value.replace(/\D/g, ''))} />
        <TextField
          label={t.startDate}
          required
          hint={t.startDateHint}
          type="date"
          value={startDate}
          error={errors.start_date}
          onChange={(e) => {
            setStartTouched(true)
            setStartDate(e.target.value)
          }}
        />
      </FormSection>
      <div className="flex justify-end gap-2">
        <ButtonLink href={listHref} variant="secondary">
          {texts.common.cancel}
        </ButtonLink>
        <Button type="submit" loading={busy} loadingText={texts.common.saving}>
          {texts.common.save}
        </Button>
      </div>
    </form>
  )
}
