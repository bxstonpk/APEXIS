'use client'

import { useState, type FormEvent } from 'react'
import { Button, ReferenceSelect } from '@/components'
import { texts } from '@/config'
import { FormBar, FormNotices, FormSection } from '@/features/forms/FormParts'
import { orNull } from '@/features/forms/useResourceForm'
import { apiRequest, errorMessage } from '@/lib/http'
import { showToast } from '@/lib/toast'
import type { ManufacturingSettings, Ref } from './types'

type Key = keyof ManufacturingSettings

/** ค่าตั้งการผลิต — PUT ทั้งชุด (ว่าง = null) */
export function MfgSettingsForm({
  apiPath,
  accountsApi,
  journalsApi,
  warehousesApi,
  canManage,
  refs,
}: {
  apiPath: string
  accountsApi: string
  journalsApi: string
  warehousesApi: string
  canManage: boolean
  refs: Record<Key, Ref | null>
}) {
  const t = texts.manufacturing.settings
  const f = t.fields
  const readOnly = !canManage
  const [v, setV] = useState(refs)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
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

  const account = (k: Key, label: string, type: string) => (
    <ReferenceSelect label={label} source={accountsApi} params={{ postable: 'true', status: 'active', type }} value={v[k]} onChange={set(k)} disabled={readOnly} texts={texts.reference} />
  )

  return (
    <form noValidate onSubmit={submit} className="flex flex-col gap-6">
      <FormNotices formError={error} readOnly={readOnly} />
      <FormSection title={t.accounts}>
        {account('wip_account_id', f.wip, 'asset')}
        {account('conversion_account_id', f.conversion, 'expense')}
        <ReferenceSelect label={f.journal} source={journalsApi} params={{ status: 'active' }} value={v.journal_id} onChange={set('journal_id')} disabled={readOnly} texts={texts.reference} />
      </FormSection>
      <FormSection title={t.defaults}>
        <ReferenceSelect
          label={f.warehouse}
          source={warehousesApi}
          params={{ status: 'active' }}
          value={v.default_warehouse_id}
          onChange={set('default_warehouse_id')}
          disabled={readOnly}
          texts={texts.reference}
        />
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
