'use client'

import { useRouter } from 'next/navigation'
import { useState, type FormEvent } from 'react'
import { Button, ButtonLink, ReferenceSelect, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { normalizeAmount, toUnits } from '@/features/accounting/money'
import { FormActions, FormBar, FormNotices, FormSection, StatusSelect } from '@/features/forms/FormParts'
import { orNull, useResourceForm } from '@/features/forms/useResourceForm'
import { today } from '@/lib/format'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, normalizeCode, requiredMessage, type FieldErrors } from '@/lib/validation'
import type { CashAccount, Ref, WhtType } from './types'

// ---------------------------------------------------------------- บัญชีเงินสด / ธนาคาร

export function CashAccountForm({
  apiPath,
  accountsApi,
  listHref,
  canManage,
  item,
  glAccount,
}: {
  apiPath: string
  accountsApi: string
  listHref: string
  canManage: boolean
  item?: CashAccount
  glAccount?: Ref | null
}) {
  const t = texts.finance.cashAccounts
  const f = t.fields
  const readOnly = !canManage
  const form = useResourceForm({ apiPath, id: item?.id, listHref })
  const [v, setV] = useState({
    code: item?.code ?? '',
    name: item?.name ?? '',
    type: item?.type ?? 'bank',
    bank_name: item?.bank_name ?? '',
    bank_branch: item?.bank_branch ?? '',
    account_no: item?.account_no ?? '',
    status: item?.status ?? 'active',
  })
  const [gl, setGl] = useState<Ref | null>(glAccount ?? null)
  const [errors, setErrors] = useState<FieldErrors<'code' | 'name' | 'gl' | 'bank_name' | 'account_no'>>({})
  const set = (k: keyof typeof v, val: string) => setV((s) => ({ ...s, [k]: val }))

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: typeof errors = {}
    if (!v.code) errs.code = requiredMessage(f.code)
    if (!v.name.trim()) errs.name = requiredMessage(f.name)
    if (!gl) errs.gl = requiredMessage(f.glAccount)
    if (v.type === 'bank' && !v.bank_name.trim()) errs.bank_name = requiredMessage(f.bankName)
    if (v.type === 'bank' && !v.account_no.trim()) errs.account_no = requiredMessage(f.accountNo)
    setErrors(errs)
    if (readOnly) return
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    const bank = v.type === 'bank'
    form.save({
      code: v.code,
      name: v.name.trim(),
      type: v.type,
      bank_name: bank ? v.bank_name.trim() : '',
      bank_branch: bank ? v.bank_branch.trim() : '',
      account_no: bank ? v.account_no.trim() : '',
      gl_account_id: gl!.id,
      status: v.status,
    })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly} />
      <FormSection title={texts.branches.form.general} description={texts.common.sections.generalHint}>
        <TextField label={f.code} required value={v.code} maxLength={30} disabled={readOnly} error={errors.code} onChange={(e) => set('code', normalizeCode(e.target.value))} />
        <TextField label={f.name} required value={v.name} maxLength={200} disabled={readOnly} error={errors.name} onChange={(e) => set('name', e.target.value)} />
        <SelectField
          label={f.type}
          value={v.type}
          disabled={readOnly}
          options={Object.entries(t.types).map(([value, label]) => ({ value, label }))}
          onChange={(e) => set('type', e.target.value)}
        />
        {item && <StatusSelect value={v.status} disabled={readOnly} onChange={(s) => set('status', s)} />}
        <ReferenceSelect
          label={f.glAccount}
          required
          hint={t.glHint}
          source={accountsApi}
          params={{ postable: 'true', status: 'active', type: 'asset' }}
          value={gl}
          onChange={setGl}
          error={errors.gl}
          disabled={readOnly}
          texts={texts.reference}
          className="sm:col-span-2"
        />
      </FormSection>
      {v.type === 'bank' && (
        <FormSection title={t.bankSection}>
          <TextField label={f.bankName} required value={v.bank_name} maxLength={100} disabled={readOnly} error={errors.bank_name} onChange={(e) => set('bank_name', e.target.value)} />
          <TextField label={f.bankBranch} value={v.bank_branch} maxLength={100} disabled={readOnly} onChange={(e) => set('bank_branch', e.target.value)} />
          <TextField
            label={f.accountNo}
            required
            value={v.account_no}
            maxLength={30}
            inputMode="numeric"
            disabled={readOnly}
            error={errors.account_no}
            onChange={(e) => set('account_no', e.target.value)}
          />
        </FormSection>
      )}
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

// ---------------------------------------------------------------- ประเภทภาษีหัก ณ ที่จ่าย

export function WhtTypeForm({ apiPath, listHref, canManage, item }: { apiPath: string; listHref: string; canManage: boolean; item?: WhtType }) {
  const t = texts.finance.whtTypes
  const f = t.fields
  const readOnly = !canManage
  const form = useResourceForm({ apiPath, id: item?.id, listHref })
  const [v, setV] = useState({ code: item?.code ?? '', name: item?.name ?? '', rate: item?.rate ?? '', status: item?.status ?? 'active' })
  const [errors, setErrors] = useState<FieldErrors<'code' | 'name' | 'rate'>>({})
  const set = (k: keyof typeof v, val: string) => setV((s) => ({ ...s, [k]: val }))

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: typeof errors = {}
    if (!v.code) errs.code = requiredMessage(f.code)
    if (!v.name.trim()) errs.name = requiredMessage(f.name)
    const rate = normalizeAmount(v.rate)
    if (!rate || toUnits(rate) <= 0n || toUnits(rate) > toUnits('100')) errs.rate = t.rateHint
    setErrors(errs)
    if (readOnly) return
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    form.save({ code: v.code, name: v.name.trim(), rate, status: v.status })
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={form.formError} readOnly={readOnly} />
      <FormSection title={texts.branches.form.general} description={t.description}>
        <TextField label={f.code} required value={v.code} maxLength={30} disabled={readOnly} error={errors.code} onChange={(e) => set('code', normalizeCode(e.target.value))} />
        <TextField label={f.name} required value={v.name} maxLength={200} disabled={readOnly} error={errors.name} onChange={(e) => set('name', e.target.value)} />
        <TextField
          label={f.rate}
          required
          hint={t.rateHint}
          inputMode="decimal"
          value={v.rate}
          disabled={readOnly}
          error={errors.rate}
          onChange={(e) => set('rate', e.target.value)}
        />
        {item && <StatusSelect value={v.status} disabled={readOnly} onChange={(s) => set('status', s)} />}
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

// ---------------------------------------------------------------- ค่าตั้งการเงิน

type SettingsKey = 'receipt_journal_id' | 'payment_journal_id' | 'transfer_journal_id' | 'wht_received_account_id' | 'wht_pnd3_account_id' | 'wht_pnd53_account_id'

export function FinanceSettingsForm({
  apiPath,
  journalsApi,
  accountsApi,
  canManage,
  refs,
}: {
  apiPath: string
  journalsApi: string
  accountsApi: string
  canManage: boolean
  /** ค่าปัจจุบัน (ชื่อจาก server) */
  refs: Record<SettingsKey, Ref | null>
}) {
  const t = texts.finance.settings
  const f = t.fields
  const readOnly = !canManage
  const [v, setV] = useState(refs)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const set = (k: SettingsKey) => (r: Ref | null) => setV((s) => ({ ...s, [k]: r }))

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (readOnly) return
    setBusy(true)
    setError(null)
    try {
      const body = Object.fromEntries(Object.entries(v).map(([k, r]) => [k, orNull(r?.id)]))
      await apiRequest(apiPath, { method: 'PUT', body })
      showToast(texts.common.toast.saved)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const journal = (k: SettingsKey, label: string) => (
    <ReferenceSelect label={label} source={journalsApi} params={{ status: 'active' }} value={v[k]} onChange={set(k)} disabled={readOnly} texts={texts.reference} />
  )
  const account = (k: SettingsKey, label: string, type: string) => (
    <ReferenceSelect label={label} source={accountsApi} params={{ postable: 'true', status: 'active', type }} value={v[k]} onChange={set(k)} disabled={readOnly} texts={texts.reference} />
  )

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={error} readOnly={readOnly} />
      <FormSection title={t.journals} description={t.journalsHint}>
        {journal('receipt_journal_id', f.receiptJournal)}
        {journal('payment_journal_id', f.paymentJournal)}
        {journal('transfer_journal_id', f.transferJournal)}
      </FormSection>
      <FormSection title={t.accounts} description={t.accountsHint}>
        {account('wht_received_account_id', f.whtReceived, 'asset')}
        {account('wht_pnd3_account_id', f.whtPnd3, 'liability')}
        {account('wht_pnd53_account_id', f.whtPnd53, 'liability')}
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

// ---------------------------------------------------------------- โอนเงิน

export function TransferForm({ apiPath, cashApi, docsPath, listHref }: { apiPath: string; cashApi: string; docsPath: string; listHref: string }) {
  const t = texts.finance
  const router = useRouter()
  const [v, setV] = useState({ date: today(), amount: '', reference: '', description: '' })
  const [from, setFrom] = useState<Ref | null>(null)
  const [to, setTo] = useState<Ref | null>(null)
  const [errors, setErrors] = useState<FieldErrors<'from' | 'to' | 'amount' | 'date'>>({})
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const set = (k: keyof typeof v, val: string) => setV((s) => ({ ...s, [k]: val }))

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    const errs: typeof errors = {}
    if (!from) errs.from = requiredMessage(t.fields.from)
    if (!to) errs.to = requiredMessage(t.fields.to)
    else if (from && from.id === to.id) errs.to = t.errors.sameAccount
    const amount = normalizeAmount(v.amount)
    if (!amount || toUnits(amount) <= 0n) errs.amount = t.errors.amountRequired
    if (!v.date) errs.date = requiredMessage(t.fields.date)
    setErrors(errs)
    if (Object.keys(errs).length) return focusFirstInvalid(e.currentTarget)
    setBusy(true)
    setError(null)
    try {
      const saved = await apiRequest<{ id: string; document_no: string }>(apiPath, {
        method: 'POST',
        body: { date: v.date, from_cash_account_id: from!.id, to_cash_account_id: to!.id, amount, reference: v.reference.trim(), description: v.description.trim() },
      })
      showToast(format(t.toast.saved, { no: saved.document_no }))
      router.push(`${docsPath}/${saved.id}`)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
      setBusy(false)
    }
  }

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={error} readOnly={false} />
      <FormSection title={t.transfers.heading} description={t.transfers.description}>
        <ReferenceSelect label={t.fields.from} required source={cashApi} params={{ status: 'active' }} value={from} onChange={setFrom} error={errors.from} texts={texts.reference} />
        <ReferenceSelect label={t.fields.to} required source={cashApi} params={{ status: 'active' }} value={to} onChange={setTo} error={errors.to} texts={texts.reference} />
        <TextField
          label={t.fields.amount}
          required
          inputMode="decimal"
          value={v.amount}
          error={errors.amount}
          className="[&_input]:text-right [&_input]:tabular-nums"
          onChange={(e) => set('amount', e.target.value)}
        />
        <TextField label={t.fields.date} required type="date" value={v.date} error={errors.date} onChange={(e) => set('date', e.target.value)} />
        <TextField label={t.fields.reference} value={v.reference} maxLength={50} onChange={(e) => set('reference', e.target.value)} />
        <TextField label={t.fields.description} value={v.description} maxLength={500} onChange={(e) => set('description', e.target.value)} />
      </FormSection>
      <FormBar>
        <p className="hidden text-[13px] text-ink-muted sm:block">{t.actions.saveHint}</p>
        <div className="ml-auto flex gap-2">
          <ButtonLink href={listHref} variant="secondary">
            {texts.common.cancel}
          </ButtonLink>
          <Button type="submit" loading={busy} loadingText={texts.common.saving}>
            {t.actions.save}
          </Button>
        </div>
      </FormBar>
    </form>
  )
}
