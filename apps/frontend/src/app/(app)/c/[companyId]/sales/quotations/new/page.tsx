import type { Metadata } from 'next'
import { texts } from '@/config'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { loadVat } from '@/features/sales/pages'
import { SalesDocForm } from '@/features/sales/SalesDocForm'
import { perms, salesApi, salesPage } from '@/features/sales/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.sales.quotations.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.quotationManage)) return <NoPermission />
  const t = texts.sales.quotations
  const path = salesPage(companyId, 'quotations')
  return (
    <FormPage crumb={{ app: texts.apps.sales.name, label: t.heading, href: path }} title={t.newTitle}>
      <SalesDocForm
        kind="quotation"
        apiPath={salesApi.quotations(companyId)}
        customersApi={salesApi.customers(companyId)}
        itemsApi={salesApi.items(companyId)}
        docsPath={path}
        listHref={path}
        vat={await loadVat(companyId)}
      />
    </FormPage>
  )
}
