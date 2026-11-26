import type { Metadata } from 'next'
import { Alert } from '@/components'
import { texts } from '@/config'
import { MfgSettingsForm } from '@/features/manufacturing/SettingsForm'
import { mfgApi, mfgPerms, type ManufacturingSettings } from '@/features/manufacturing/types'
import { FormPage } from '@/features/organization/pages'
import { errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { loadRef } from '@/lib/server/list'

export const metadata: Metadata = { title: texts.manufacturing.settings.pageTitle }

/** ค่าตั้งการผลิต — ชื่อบัญชี / สมุด / คลังที่ตั้งไว้โหลดพร้อมกัน (4 คำขอขนานกัน) */
export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.manufacturing.settings
  const res = await backendGet<ManufacturingSettings>(mfgApi.settings(companyId)).then(
    (r) => ({ data: r.data }),
    (err: unknown) => ({ error: errorMessage(err) }),
  )
  if ('error' in res) return <Alert tone="error">{res.error}</Alert>
  const s = res.data
  const [journal, wip, conversion, warehouse] = await Promise.all([
    loadRef(mfgApi.journals(companyId), s.journal_id ?? ''),
    loadRef(mfgApi.accounts(companyId), s.wip_account_id ?? ''),
    loadRef(mfgApi.accounts(companyId), s.conversion_account_id ?? ''),
    loadRef(mfgApi.warehouses(companyId), s.default_warehouse_id ?? ''),
  ])
  return (
    <FormPage title={t.heading} description={t.description}>
      <MfgSettingsForm
        apiPath={mfgApi.settings(companyId)}
        accountsApi={mfgApi.accounts(companyId)}
        journalsApi={mfgApi.journals(companyId)}
        warehousesApi={mfgApi.warehouses(companyId)}
        canManage={can(ctx.permissions, mfgPerms.settingsManage)}
        refs={{ journal_id: journal, wip_account_id: wip, conversion_account_id: conversion, default_warehouse_id: warehouse }}
      />
    </FormPage>
  )
}
