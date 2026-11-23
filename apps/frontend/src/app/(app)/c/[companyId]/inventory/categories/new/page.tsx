import type { Metadata } from 'next'
import { texts } from '@/config'
import { CategoryForm } from '@/features/inventory/MasterForms'
import { invApi, inventoryPage, invPerms } from '@/features/inventory/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.inventory.categories.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, invPerms.categoryManage)) return <NoPermission />
  const t = texts.inventory.categories
  const path = inventoryPage(companyId, 'categories')
  return (
    <FormPage crumb={{ app: texts.apps.inventory.name, label: t.heading, href: path }} title={t.newTitle}>
      <CategoryForm apiPath={invApi.categories(companyId)} accountsApi={invApi.accounts(companyId)} listHref={path} canManage />
    </FormPage>
  )
}
