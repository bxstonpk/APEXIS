import type { Metadata } from 'next'
import { Alert } from '@/components'
import { texts } from '@/config'
import { InventorySettingsForm } from '@/features/inventory/SettingsForm'
import { invApi, invPerms, type InventorySettings } from '@/features/inventory/types'
import { FormPage } from '@/features/organization/pages'
import { errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { loadRef } from '@/lib/server/list'

export const metadata: Metadata = { title: texts.inventory.settings.pageTitle }

/** ค่าตั้งคลังสินค้า — ชื่อบัญชี / สมุดที่ตั้งไว้โหลดพร้อมกัน (4 คำขอขนานกัน) */
export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.inventory.settings
  const res = await backendGet<InventorySettings>(invApi.settings(companyId)).then(
    (r) => r.data,
    (err: unknown) => err,
  )
  if (!res || typeof res !== 'object' || !('accounting_systems' in res)) return <Alert tone="error">{errorMessage(res)}</Alert>
  const s = res as InventorySettings
  const accounts = invApi.accounts(companyId)
  const [purchase, inventory, cogs, journal] = await Promise.all([
    loadRef(accounts, s.default_purchase_account_id ?? ''),
    loadRef(accounts, s.default_inventory_account_id ?? ''),
    loadRef(accounts, s.cogs_account_id ?? ''),
    loadRef(invApi.journals(companyId), s.journal_id ?? ''),
  ])
  return (
    <FormPage title={t.heading} description={t.description}>
      <InventorySettingsForm
        apiPath={invApi.settings(companyId)}
        systemApi={`${invApi.settings(companyId)}/accounting-systems`}
        accountsApi={accounts}
        journalsApi={invApi.journals(companyId)}
        canManage={can(ctx.permissions, invPerms.settingsManage)}
        settings={s}
        refs={{ default_purchase_account_id: purchase, default_inventory_account_id: inventory, cogs_account_id: cogs, journal_id: journal }}
      />
    </FormPage>
  )
}
