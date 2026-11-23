import type { Metadata } from 'next'
import { texts } from '@/config'
import { UomForm } from '@/features/inventory/MasterForms'
import { invApi, inventoryPage, invPerms } from '@/features/inventory/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.inventory.uoms.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, invPerms.uomManage)) return <NoPermission />
  const t = texts.inventory.uoms
  const path = inventoryPage(companyId, 'uoms')
  return (
    <FormPage crumb={{ app: texts.apps.inventory.name, label: t.heading, href: path }} title={t.newTitle}>
      <UomForm apiPath={invApi.uoms(companyId)} listHref={path} canManage />
    </FormPage>
  )
}
