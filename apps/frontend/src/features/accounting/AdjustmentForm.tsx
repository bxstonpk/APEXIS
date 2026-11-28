'use client'

import { useRouter } from 'next/navigation'
import { useState, type FormEvent } from 'react'
import { Button, ButtonLink, ReferenceSelect, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { FormBar, FormNotices, FormSection } from '@/features/forms/FormParts'
import { formatDate } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid } from '@/lib/validation'
import { normalizeAmount, toUnits } from './money'
import { ADJUSTMENT_TEMPLATES, type AdjustmentTemplate, type Entry, type Ref } from './types'

/** วันสุดท้ายของเดือน (YYYY-MM -> YYYY-MM-DD) */
export function monthEnd(month: string): string {
  const [y, m] = month.split('-').map(Number)
  if (!y || !m) return ''
  const d = new Date(Date.UTC(y, m, 0))
  return d.toISOString().slice(0, 10)
}

/** บัญชีตั้งต้นของแบบ: [เดบิต, เครดิต] */
export function templateAccounts(t: AdjustmentTemplate | undefined): [Ref | null, Ref | null] {
  if (!t) return [null, null]
  return t.balance_debit ? [t.balance_account, t.result_account] : [t.result_account, t.balance_account]
}

export type AdjustmentState = {
  template: string
  period: string // 'adj:<year>' = งวดปรับปรุง, 'month' = สิ้นเดือน
  month: string
  journal: Ref | null
  debit: Ref | null
  credit: Ref | null
  amount: string
  description: string
  reference: string
  autoReverse: boolean
}

export type OpenYear = { fiscal_year: number; end_date: string }

/** ตรวจ + สร้าง body ของ POST /adjusting-entries (pure) */
export function buildAdjustment(s: AdjustmentState, years: OpenYear[]): { errors: Record<string, string>; body?: Record<string, unknown> } {
  const e = texts.adjustments.errors
  const errors: Record<string, string> = {}
  const year = s.period.startsWith('adj:') ? years.find((y) => `adj:${y.fiscal_year}` === s.period) : undefined
  const date = year ? year.end_date : monthEnd(s.month)
  if (!year && !date) errors.month = e.monthRequired
  if (!s.journal) errors.journal = e.journalRequired
  if (!s.debit) errors.debit = e.accountRequired
  if (!s.credit) errors.credit = e.accountRequired
  if (s.debit && s.credit && s.debit.id === s.credit.id) errors.credit = e.sameAccount
  const amount = normalizeAmount(s.amount)
  if (!amount || toUnits(amount) <= 0n) errors.amount = e.amountInvalid
  if (Object.keys(errors).length) return { errors }
  return {
    errors,
    body: {
      journal_id: s.journal!.id,
      date,
      adjusting_period: !!year,
      template: s.template,
      auto_reverse: s.autoReverse,
      description: s.description.trim(),
      reference: s.reference.trim(),
      lines: [
        { account_id: s.debit!.id, debit: amount, credit: '0' },
        { account_id: s.credit!.id, debit: '0', credit: amount },
      ],
    },
  }
}

type Props = {
  api: { adjustments: string; accounts: string; journals: string }
  entriesPath: string
  listHref: string
  templates: AdjustmentTemplate[]
  /** ปีที่ยังไม่ปิดบัญชี (ล่าสุดก่อน) — ค่าตั้งต้นของงวด = งวดปรับปรุงของปีล่าสุด */
  openYears: OpenYear[]
  journal: Ref | null
}

export function AdjustmentForm({ api, entriesPath, listHref, templates, openYears, journal }: Props) {
  const t = texts.adjustments
  const f = t.fields
  const router = useRouter()
  const byKey = (k: string) => templates.find((x) => x.template === k)
  const first = ADJUSTMENT_TEMPLATES[0]
  const [debit0, credit0] = templateAccounts(byKey(first))
  const [s, setS] = useState<AdjustmentState>({
    template: first,
    period: openYears[0] ? `adj:${openYears[0].fiscal_year}` : 'month',
    month: '',
    journal,
    debit: debit0,
    credit: credit0,
    amount: '',
    description: '',
    reference: '',
    autoReverse: byKey(first)?.auto_reverse ?? true,
  })
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const set = (patch: Partial<AdjustmentState>) => setS((x) => ({ ...x, ...patch }))

  function pickTemplate(key: string) {
    const tpl = byKey(key)
    const [d, c] = templateAccounts(tpl)
    set({ template: key, debit: d ?? s.debit, credit: c ?? s.credit, autoReverse: tpl?.auto_reverse ?? key !== 'other' })
  }

  async function submit(ev: FormEvent<HTMLFormElement>) {
    ev.preventDefault()
    const { errors: errs, body } = buildAdjustment(s, openYears)
    setErrors(errs)
    if (!body) {
      focusFirstInvalid(ev.currentTarget)
      return
    }
    setBusy(true)
    setFormError(null)
    try {
      const saved = await apiRequest<Entry>(api.adjustments, { method: 'POST', body })
      showToast(format(t.created, { no: saved.entry_no ?? '' }))
      router.push(`${entriesPath}/${saved.id}`)
      router.refresh()
    } catch (err) {
      setFormError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const periodOptions = [
    ...openYears.map((y) => ({ value: `adj:${y.fiscal_year}`, label: format(f.adjustingPeriod, { year: String(y.fiscal_year + 543), date: formatDate(y.end_date) }) })),
    { value: 'month', label: f.monthEnd },
  ]

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-8">
      <FormNotices formError={formError} readOnly={false} />
      <FormSection title={f.template}>
        <SelectField
          label={f.template}
          value={s.template}
          onChange={(e) => pickTemplate(e.target.value)}
          options={ADJUSTMENT_TEMPLATES.map((k) => ({ value: k, label: t.templates[k] ?? k }))}
        />
        <SelectField label={f.period} value={s.period} onChange={(e) => set({ period: e.target.value })} options={periodOptions} />
        {s.period === 'month' && (
          <TextField label={f.month} required type="month" value={s.month} error={errors.month} onChange={(e) => set({ month: e.target.value })} />
        )}
        <ReferenceSelect
          label={f.journal}
          required
          source={api.journals}
          params={{ status: 'active' }}
          value={s.journal}
          onChange={(v) => set({ journal: v })}
          error={errors.journal}
          texts={texts.reference}
        />
      </FormSection>
      <FormSection title={texts.entries.sections.lines}>
        <ReferenceSelect
          label={f.debitAccount}
          required
          source={api.accounts}
          params={{ postable: 'true', status: 'active' }}
          value={s.debit}
          onChange={(v) => set({ debit: v })}
          error={errors.debit}
          texts={texts.reference}
        />
        <ReferenceSelect
          label={f.creditAccount}
          required
          source={api.accounts}
          params={{ postable: 'true', status: 'active' }}
          value={s.credit}
          onChange={(v) => set({ credit: v })}
          error={errors.credit}
          texts={texts.reference}
        />
        <TextField label={f.amount} required inputMode="decimal" value={s.amount} error={errors.amount} onChange={(e) => set({ amount: e.target.value })} className="text-right" />
        <TextField label={f.description} value={s.description} maxLength={500} onChange={(e) => set({ description: e.target.value })} />
        <TextField label={f.reference} value={s.reference} maxLength={100} onChange={(e) => set({ reference: e.target.value })} />
        <label className="flex cursor-pointer items-start gap-3">
          <input type="checkbox" checked={s.autoReverse} onChange={(e) => set({ autoReverse: e.target.checked })} className="mt-0.5 size-4 accent-primary" />
          <span className="text-sm font-medium">{f.autoReverse}</span>
        </label>
      </FormSection>
      <FormBar>
        <div className="ml-auto flex gap-2">
          <ButtonLink href={listHref} variant="secondary">
            {texts.common.cancel}
          </ButtonLink>
          <Button type="submit" loading={busy} loadingText={t.saving}>
            {t.submit}
          </Button>
        </div>
      </FormBar>
    </form>
  )
}
