'use client'

import { useRouter } from 'next/navigation'
import { useId, useState, type FormEvent } from 'react'
import { Alert, Button, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, normalizeAmount, toUnits } from '@/features/accounting/money'
import { formatDate, today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { requiredMessage } from '@/lib/validation'
import { monthLabel, type TaxFiling, type TaxForm } from './tax'

type Fields = { filed_date: string; amount: string; surcharge: string; penalty: string; carry_forward: string; reference: string }
type Errors = Partial<Record<keyof Fields, string>>

/** ตรวจฟอร์มบันทึกการยื่น — คืน body ที่ส่ง API ได้ หรือ error ต่อช่อง */
export function validateFiling(form: TaxForm, month: string, v: Fields): { body?: Record<string, string>; errors: Errors } {
  const t = texts.finance.taxFilings.filing
  const errors: Errors = {}
  if (!v.filed_date) errors.filed_date = requiredMessage(t.filedDate)
  const amounts: Record<string, string> = {}
  for (const key of ['amount', 'surcharge', 'penalty', 'carry_forward'] as const) {
    if (key === 'carry_forward' && form !== 'pp30') continue
    const n = normalizeAmount(v[key])
    if (n === null || (n !== '' && toUnits(n) < 0n)) errors[key] = t.amountInvalid
    else amounts[key] = n === '' ? '0' : n
  }
  if (!v.amount.trim() && !errors.amount) errors.amount = requiredMessage(t.amount)
  if (Object.keys(errors).length > 0) return { errors }
  return { errors, body: { form, month, filed_date: v.filed_date, reference: v.reference.trim(), ...amounts } }
}

/**
 * การยื่นแบบของเดือน (บันทึกเท่านั้น — การจ่ายภาษีทำที่หน้าจ่ายเงิน)
 * - ยื่นแล้ว: แสดงวันที่ / ยอด / เลขอ้างอิง + ยกเลิกบันทึก (ต้องมีสิทธิ์ manage)
 * - ยังไม่ยื่น: ฟอร์มบันทึก (ภาษีตั้งต้นจากแบบ; ภ.พ.30 ชำระเกิน = ยกไปเดือนถัดไป)
 */
export function TaxFilingPanel({
  apiPath,
  form,
  month,
  filing,
  suggestedAmount,
  suggestedCarry,
  canManage,
}: {
  apiPath: string
  form: TaxForm
  month: string
  filing: TaxFiling | null
  suggestedAmount: string
  suggestedCarry?: string
  canManage: boolean
}) {
  const t = texts.finance.taxFilings
  const f = t.filing
  const router = useRouter()
  const headingId = useId()
  const [v, setV] = useState<Fields>({
    filed_date: today(),
    amount: suggestedAmount,
    surcharge: '',
    penalty: '',
    carry_forward: suggestedCarry ?? '',
    reference: '',
  })
  const [errors, setErrors] = useState<Errors>({})
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const set = (k: keyof Fields) => (e: { target: { value: string } }) => setV((x) => ({ ...x, [k]: e.target.value }))

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const res = validateFiling(form, month, v)
    setErrors(res.errors)
    if (!res.body) return
    setBusy(true)
    setError(null)
    try {
      await apiRequest(apiPath, { method: 'POST', body: res.body })
      showToast(f.saved)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  async function cancel() {
    if (!filing || !window.confirm(format(t.actions.confirmCancel, { form: t.forms[form]!, month: monthLabel(month) }))) return
    setBusy(true)
    setError(null)
    try {
      await apiRequest(`${apiPath}/${filing.id}`, { method: 'PATCH', body: { status: 'cancelled' } })
      showToast(f.cancelled)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section aria-labelledby={headingId} className="rounded-(--radius-panel) border border-line bg-surface p-5 shadow-card print:hidden">
      <h2 id={headingId} className="text-[15px] font-semibold">
        {f.title}
      </h2>
      {error && (
        <div className="mt-3">
          <Alert tone="error">{error}</Alert>
        </div>
      )}
      {filing ? (
        <div className="mt-3 flex flex-wrap items-end justify-between gap-4">
          <dl className="grid grid-cols-2 gap-x-8 gap-y-2 text-sm sm:grid-cols-4">
            <Fact label={f.filedDate} value={formatDate(filing.filed_date)} />
            <Fact label={f.amount} value={formatAmount(filing.amount)} />
            {toUnits(filing.surcharge) + toUnits(filing.penalty) > 0n && <Fact label={f.total} value={formatAmount(filing.total)} />}
            {form === 'pp30' && toUnits(filing.carry_forward) > 0n && <Fact label={f.carryForward} value={formatAmount(filing.carry_forward)} />}
            {filing.reference && <Fact label={f.reference} value={filing.reference} />}
          </dl>
          {canManage && (
            <Button variant="secondary" onClick={cancel} loading={busy} loadingText={t.actions.working}>
              {t.actions.cancel}
            </Button>
          )}
        </div>
      ) : canManage ? (
        <form noValidate onSubmit={submit} className="mt-4 grid gap-3 sm:grid-cols-3">
          <TextField label={f.filedDate} required type="date" value={v.filed_date} error={errors.filed_date} onChange={set('filed_date')} />
          <TextField label={f.amount} required inputMode="decimal" value={v.amount} error={errors.amount} onChange={set('amount')} />
          {form === 'pp30' && (
            <TextField label={f.carryForward} inputMode="decimal" value={v.carry_forward} error={errors.carry_forward} onChange={set('carry_forward')} />
          )}
          <TextField label={f.surcharge} inputMode="decimal" value={v.surcharge} error={errors.surcharge} onChange={set('surcharge')} />
          <TextField label={f.penalty} inputMode="decimal" value={v.penalty} error={errors.penalty} onChange={set('penalty')} />
          <TextField label={f.reference} value={v.reference} maxLength={100} onChange={set('reference')} />
          <div className="flex justify-end sm:col-span-3">
            <Button type="submit" loading={busy} loadingText={t.actions.working}>
              {t.actions.record}
            </Button>
          </div>
        </form>
      ) : (
        <p className="mt-2 text-sm text-ink-muted">{t.status.notFiled}</p>
      )}
    </section>
  )
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-[12px] text-ink-muted">{label}</dt>
      <dd className="font-medium tabular-nums">{value}</dd>
    </div>
  )
}
