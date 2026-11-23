import type { Metadata } from 'next'
import { texts } from '@/config'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { vatRegistrantsApi } from '@/features/partners/thaiAddress'
import { CustomerForm } from '@/features/sales/Forms'
import { perms, salesApi, salesPage } from '@/features/sales/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.sales.customers.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.customerManage)) return <NoPermission />
  const t = texts.sales.customers
  const path = salesPage(companyId, 'customers')
  return (
    <FormPage crumb={{ app: texts.apps.sales.name, label: t.heading, href: path }} title={t.newTitle}>
      <CustomerForm apiPath={salesApi.customers(companyId)} lookupApi={vatRegistrantsApi(companyId)} customersPath={salesPage(companyId, 'customers')} listHref={path} canManage />
    </FormPage>
  )
}
