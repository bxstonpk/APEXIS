import type { Metadata } from 'next'
import { texts } from '@/config'
import { MoneyDocForm } from '@/features/finance/MoneyDocForm'
import { financeApi, financePage, perms } from '@/features/finance/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.finance.receipts.newTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/**
 * บันทึกรับเงิน — ?customer_id=&billing_note_id= (จากเอกสารขาย / ใบวางบิล) เลือกลูกค้าและใบแจ้งหนี้ตามใบวางบิลให้
 * การรับเงินของเอกสารขายทั้งหมดยืนยันที่นี่ (เลือกบัญชีเงินสด / ธนาคารที่เงินเข้า)
 */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.receiptPost)) return <NoPermission />
  const path = financePage(companyId, 'receipts')
  const customer = await loadRef(financeApi.customers(companyId), param(sp, 'customer_id'))
  return (
    <FormPage crumb={{ app: texts.apps.finance.name, label: texts.finance.receipts.heading, href: path }} title={texts.finance.receipts.newTitle}>
      <MoneyDocForm
        mode="receipt"
        docsPath={path}
        listHref={path}
        initialParty={customer}
        initialBillingNote={customer ? param(sp, 'billing_note_id') : ''}
        api={{
          submit: financeApi.receipts(companyId),
          open: `${financeApi.receipts(companyId)}/open-invoices`,
          parties: financeApi.customers(companyId),
          accounts: financeApi.accounts(companyId),
          cashAccounts: financeApi.cashAccounts(companyId),
          billingNotes: `/companies/${companyId}/billing-notes`,
        }}
      />
    </FormPage>
  )
}
