'use client'

import { useState, type FormEvent } from 'react'
import { Button, ReferenceSelect, TextField } from '@/components'
import { texts } from '@/config'
import { normalizeAmount, toUnits } from '@/features/accounting/money'
import { FormBar, FormNotices, FormSection } from '@/features/forms/FormParts'
import { orNull } from '@/features/forms/useResourceForm'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import { focusFirstInvalid, requiredMessage } from '@/lib/validation'
import type { PurchasingSettings, Ref } from './types'

type AccountKey =
  | 'journal_id'
  | 'grir_account_id'
  | 'ap_account_id'
  | 'input_vat_account_id'
  | 'undue_input_vat_account_id'
  | 'price_variance_account_id'
  | 'purchase_returns_account_id'
  | 'service_expense_account_id'

/** ค่าตั้งการจัดซื้อ — อัตรา VAT (ใช้เมื่อผู้ขายจด VAT) + สมุด / บัญชีที่ลงรายการ */
export function PurchasingSettingsForm({
  apiPath,
  journalsApi,
  accountsApi,
  canManage,
  settings,
  refs,
}: {
  apiPath: string
  journalsApi: string
  accountsApi: string
  canManage: boolean
  settings: PurchasingSettings
  refs: Record<AccountKey, Ref | null>
}) {
  const t = texts.purchasing.settings
  const f = t.fields
  const readOnly = !canManage
  const [rate, setRate] = useState(settings.vat_rate)
  const [v, setV] = useState(refs)
  const [rateError, setRateError] = useState<string>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const set = (k: AccountKey) => (r: Ref | null) => setV((s) => ({ ...s, [k]: r }))

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (readOnly) return
    const n = normalizeAmount(rate)
    if (n === null || n === '' || toUnits(n) < 0n || toUnits(n) >= toUnits('100')) {
      setRateError(requiredMessage(f.vatRate))
      return focusFirstInvalid(e.currentTarget)
    }
    setRateError(undefined)
    setBusy(true)
    setError(null)
    try {
      await apiRequest(apiPath, { method: 'PUT', body: { vat_rate: n, ...Object.fromEntries(Object.entries(v).map(([k, r]) => [k, orNull(r?.id)])) } })
      showToast(texts.common.toast.saved)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const account = (k: AccountKey, label: string, type: string) => (
    <ReferenceSelect label={label} source={accountsApi} params={{ postable: 'true', status: 'active', type }} value={v[k]} onChange={set(k)} disabled={readOnly} texts={texts.reference} />
  )

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={error} readOnly={readOnly} />
      <FormSection title={t.vat}>
        <TextField label={f.vatRate} required inputMode="decimal" value={rate} disabled={readOnly} error={rateError} onChange={(e) => setRate(e.target.value)} />
      </FormSection>
      <FormSection title={t.accounts}>
        <ReferenceSelect label={f.journal} source={journalsApi} params={{ status: 'active' }} value={v.journal_id} onChange={set('journal_id')} disabled={readOnly} texts={texts.reference} />
        {account('grir_account_id', f.grir, 'liability')}
        {account('ap_account_id', f.ap, 'liability')}
        {account('input_vat_account_id', f.inputVat, 'asset')}
        {account('undue_input_vat_account_id', f.undueInputVat, 'asset')}
        {account('price_variance_account_id', f.priceVariance, 'expense')}
        {account('purchase_returns_account_id', f.purchaseReturns, 'expense')}
        {account('service_expense_account_id', f.serviceExpense, 'expense')}
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
