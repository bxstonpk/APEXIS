import type { Metadata } from 'next'
import { texts } from '@/config'
import { StockDocForm } from '@/features/inventory/DocForm'
import { invApi, inventoryPage, invPerms } from '@/features/inventory/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.inventory.documents.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, invPerms.documentManage)) return <NoPermission />
  const t = texts.inventory.documents
  const path = inventoryPage(companyId, 'documents')
  return (
    <FormPage crumb={{ app: texts.apps.inventory.name, label: t.heading, href: path }} title={t.newTitle}>
      <StockDocForm
        apiPath={invApi.documents(companyId)}
        itemsApi={invApi.items(companyId)}
        warehousesApi={invApi.warehouses(companyId)}
        docsPath={path}
        listHref={path}
        canPost={can(ctx.permissions, invPerms.documentPost)}
      />
    </FormPage>
  )
}
