import type { Metadata } from 'next'
import { Alert } from '@/components'
import { texts } from '@/config'
import { FormPage } from '@/features/organization/pages'
import { PurchasingSettingsForm } from '@/features/purchasing/SettingsForm'
import { docPerms, purchasingDocsApi, type PurchasingSettings } from '@/features/purchasing/types'
import { errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { loadRef } from '@/lib/server/list'

export const metadata: Metadata = { title: texts.purchasing.settings.pageTitle }

/** ค่าตั้งการจัดซื้อ — ชื่อสมุด / บัญชีที่ตั้งไว้โหลดพร้อมกัน (5 คำขอขนานกัน) */
export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.purchasing.settings
  const res = await backendGet<PurchasingSettings>(purchasingDocsApi.settings(companyId)).then(
    (r) => r.data,
    (err: unknown) => err,
  )
  if (!res || typeof res !== 'object' || !('vat_rate' in res)) return <Alert tone="error">{errorMessage(res)}</Alert>
  const s = res as PurchasingSettings
  const accounts = purchasingDocsApi.accounts(companyId)
  const [journal, grir, ap, inputVat, variance, returns] = await Promise.all([
    loadRef(purchasingDocsApi.journals(companyId), s.journal_id ?? ''),
    loadRef(accounts, s.grir_account_id ?? ''),
    loadRef(accounts, s.ap_account_id ?? ''),
    loadRef(accounts, s.input_vat_account_id ?? ''),
    loadRef(accounts, s.price_variance_account_id ?? ''),
    loadRef(accounts, s.purchase_returns_account_id ?? ''),
  ])
  return (
    <FormPage title={t.heading} description={t.description}>
      <PurchasingSettingsForm
        apiPath={purchasingDocsApi.settings(companyId)}
        journalsApi={purchasingDocsApi.journals(companyId)}
        accountsApi={accounts}
        canManage={can(ctx.permissions, docPerms.settingsManage)}
        settings={s}
        refs={{ journal_id: journal, grir_account_id: grir, ap_account_id: ap, input_vat_account_id: inputVat, price_variance_account_id: variance, purchase_returns_account_id: returns }}
      />
    </FormPage>
  )
}
