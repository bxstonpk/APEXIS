import type { Metadata } from 'next'
import { Alert } from '@/components'
import { texts } from '@/config'
import { FormPage } from '@/features/organization/pages'
import { SalesSettingsForm } from '@/features/sales/Forms'
import { perms, salesApi, type SalesSettings } from '@/features/sales/types'
import { errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { loadRef } from '@/lib/server/list'

export const metadata: Metadata = { title: texts.sales.settings.pageTitle }

/** ค่าตั้งการขาย — ชื่อสมุด / บัญชีที่ตั้งไว้โหลดพร้อมกัน (5 คำขอขนานกัน) */
export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales.settings
  const res = await backendGet<SalesSettings>(salesApi.settings(companyId)).then(
    (r) => r.data,
    (err: unknown) => err,
  )
  if (!res || typeof res !== 'object' || !('vat_registered' in res)) return <Alert tone="error">{errorMessage(res)}</Alert>
  const s = res as SalesSettings
  const accounts = salesApi.accounts(companyId)
  const [journal, ar, outputVat, revenue, cogs, returns] = await Promise.all([
    loadRef(salesApi.journals(companyId), s.journal_id ?? ''),
    loadRef(accounts, s.ar_account_id ?? ''),
    loadRef(accounts, s.output_vat_account_id ?? ''),
    loadRef(accounts, s.revenue_account_id ?? ''),
    loadRef(accounts, s.cogs_account_id ?? ''),
    loadRef(accounts, s.sales_returns_account_id ?? ''),
  ])
  return (
    <FormPage title={t.heading} description={t.description}>
      <SalesSettingsForm
        apiPath={salesApi.settings(companyId)}
        journalsApi={salesApi.journals(companyId)}
        accountsApi={accounts}
        canManage={can(ctx.permissions, perms.settingsManage)}
        settings={s}
        refs={{ journal_id: journal, ar_account_id: ar, output_vat_account_id: outputVat, revenue_account_id: revenue, cogs_account_id: cogs, sales_returns_account_id: returns }}
      />
    </FormPage>
  )
}
