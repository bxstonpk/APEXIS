import type { Metadata } from 'next'
import { texts } from '@/config'
import { ItemForm } from '@/features/inventory/MasterForms'
import { invApi, inventoryPage, invPerms } from '@/features/inventory/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.inventory.items.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, invPerms.itemManage)) return <NoPermission />
  const t = texts.inventory.items
  const path = inventoryPage(companyId, 'items')
  return (
    <FormPage crumb={{ app: texts.apps.inventory.name, label: t.heading, href: path }} title={t.newTitle}>
      <ItemForm apiPath={invApi.items(companyId)} uomsApi={invApi.uoms(companyId)} categoriesApi={invApi.categories(companyId)} accountsApi={invApi.accounts(companyId)} listHref={path} canManage />
    </FormPage>
  )
}
