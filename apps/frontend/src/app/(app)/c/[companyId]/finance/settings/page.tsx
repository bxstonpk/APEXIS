import type { Metadata } from 'next'
import { Alert } from '@/components'
import { texts } from '@/config'
import { FinanceSettingsForm } from '@/features/finance/Forms'
import { financeApi, perms, type FinanceSettings } from '@/features/finance/types'
import { FormPage } from '@/features/organization/pages'
import { errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { loadRef } from '@/lib/server/list'

export const metadata: Metadata = { title: texts.finance.settings.pageTitle }

/** ค่าตั้งการเงิน — ชื่อของสมุด / บัญชีที่ตั้งไว้โหลดพร้อมกัน (6 คำขอขนานกัน ไม่ใช่ N+1 ต่อแถว) */
export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.finance.settings
  const res = await backendGet<FinanceSettings>(financeApi.settings(companyId)).then(
    (r) => r.data,
    (err: unknown) => err,
  )
  if (!res || typeof res !== 'object' || !('receipt_journal_id' in res)) {
    return <Alert tone="error">{errorMessage(res)}</Alert>
  }
  const s = res as FinanceSettings
  const journals = financeApi.journals(companyId)
  const accounts = financeApi.accounts(companyId)
  const [rj, pj, tj, wr, w3, w53] = await Promise.all([
    loadRef(journals, s.receipt_journal_id ?? ''),
    loadRef(journals, s.payment_journal_id ?? ''),
    loadRef(journals, s.transfer_journal_id ?? ''),
    loadRef(accounts, s.wht_received_account_id ?? ''),
    loadRef(accounts, s.wht_pnd3_account_id ?? ''),
    loadRef(accounts, s.wht_pnd53_account_id ?? ''),
  ])
  return (
    <FormPage title={t.heading}>
      <FinanceSettingsForm
        apiPath={financeApi.settings(companyId)}
        journalsApi={journals}
        accountsApi={accounts}
        canManage={can(ctx.permissions, perms.settingsManage)}
        refs={{
          receipt_journal_id: rj, payment_journal_id: pj, transfer_journal_id: tj,
          wht_received_account_id: wr, wht_pnd3_account_id: w3, wht_pnd53_account_id: w53,
        }}
      />
    </FormPage>
  )
}
