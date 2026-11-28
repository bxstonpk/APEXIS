'use client'

import { useRouter } from 'next/navigation'
import { useState, type FormEvent } from 'react'
import { Button, ReferenceSelect, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { FormBar, FormNotices, FormSection } from '@/features/forms/FormParts'
import { orNull } from '@/features/forms/useResourceForm'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import type { InventorySettings, Ref } from './types'

type Key = 'default_purchase_account_id' | 'default_inventory_account_id' | 'cogs_account_id' | 'journal_id'

/**
 * ค่าตั้งคลังสินค้า — บัญชีค่าเริ่มต้น (ใช้เมื่อสินค้า / หมวดไม่ได้ตั้ง) + สมุดรายวัน (PUT ทั้งชุด, ว่าง = null)
 * ระบบบัญชีสินค้าต่อปีบัญชี: PUT /inventory-settings/accounting-systems/{year} {system} ทีละปี
 */
export function InventorySettingsForm({
  apiPath,
  systemApi,
  accountsApi,
  journalsApi,
  canManage,
  settings,
  refs,
}: {
  apiPath: string
  /** base path ของ accounting-systems (ต่อท้ายด้วย /{year}) */
  systemApi: string
  accountsApi: string
  journalsApi: string
  canManage: boolean
  settings: InventorySettings
  refs: Record<Key, Ref | null>
}) {
  const t = texts.inventory.settings
  const f = t.fields
  const router = useRouter()
  const readOnly = !canManage
  const [v, setV] = useState(refs)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [sys, setSys] = useState({ year: String(new Date().getFullYear()), system: 'periodic' })
  const [sysBusy, setSysBusy] = useState(false)
  const [sysError, setSysError] = useState<string>()
  const set = (k: Key) => (r: Ref | null) => setV((s) => ({ ...s, [k]: r }))

  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (readOnly) return
    setBusy(true)
    setError(null)
    try {
      await apiRequest(apiPath, { method: 'PUT', body: Object.fromEntries(Object.entries(v).map(([k, r]) => [k, orNull(r?.id)])) })
      showToast(texts.common.toast.saved)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  async function setSystem() {
    const year = Number(sys.year)
    if (!/^\d{4}$/.test(sys.year) || year < 1900) {
      setSysError(t.year)
      return
    }
    setSysError(undefined)
    setSysBusy(true)
    setError(null)
    try {
      await apiRequest(`${systemApi}/${year}`, { method: 'PUT', body: { system: sys.system } })
      showToast(texts.common.toast.saved)
      router.refresh()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setSysBusy(false)
    }
  }

  const account = (k: Key, label: string, type: string) => (
    <ReferenceSelect label={label} source={accountsApi} params={{ postable: 'true', status: 'active', type }} value={v[k]} onChange={set(k)} disabled={readOnly} texts={texts.reference} />
  )
  const systems = [...settings.accounting_systems].sort((a, b) => a.from_fiscal_year - b.from_fiscal_year)

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={error} readOnly={readOnly} />
      <FormSection title={t.accounts}>
        {account('default_purchase_account_id', f.purchase, 'expense')}
        {account('default_inventory_account_id', f.inventory, 'asset')}
        {account('cogs_account_id', f.cogs, 'expense')}
        <ReferenceSelect label={f.journal} source={journalsApi} params={{ status: 'active' }} value={v.journal_id} onChange={set('journal_id')} disabled={readOnly} texts={texts.reference} />
      </FormSection>
      <FormSection title={t.systems}>
        <ul className="flex flex-col gap-1 text-sm sm:col-span-2">
          {systems.length === 0 ? (
            <li className="text-ink-muted">{t.noSystems}</li>
          ) : (
            systems.map((s) => (
              <li key={s.from_fiscal_year} className="flex justify-between gap-3 border-b border-line py-1.5 last:border-b-0">
                <span>{format(t.fromYear, { year: String(s.from_fiscal_year) })}</span>
                <span className="font-medium">{t.systemNames[s.system] ?? s.system}</span>
              </li>
            ))
          )}
        </ul>
        {canManage && (
          <div className="flex flex-wrap items-end gap-2 sm:col-span-2">
            <TextField
              label={t.year}
              inputMode="numeric"
              maxLength={4}
              value={sys.year}
              error={sysError}
              className="w-32"
              onChange={(e) => setSys((s) => ({ ...s, year: e.target.value.replace(/\D/g, '') }))}
            />
            <SelectField
              label={t.systems}
              value={sys.system}
              options={Object.entries(t.systemNames).map(([value, label]) => ({ value, label }))}
              onChange={(e) => setSys((s) => ({ ...s, system: e.target.value }))}
              className="w-56"
            />
            <Button type="button" variant="secondary" loading={sysBusy} loadingText={texts.common.saving} onClick={() => void setSystem()}>
              {t.setSystem}
            </Button>
          </div>
        )}
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
