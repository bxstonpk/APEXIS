import type { Metadata } from 'next'
import { texts } from '@/config'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { vatRegistrantsApi } from '@/features/partners/thaiAddress'
import { SupplierForm } from '@/features/purchasing/SupplierForm'
import { perms, purchasingApi, purchasingPage } from '@/features/purchasing/types'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.purchasing.suppliers.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.supplierManage)) return <NoPermission />
  const t = texts.purchasing.suppliers
  const path = purchasingPage(companyId, 'suppliers')
  return (
    <FormPage crumb={{ app: texts.apps.purchasing.name, label: t.heading, href: path }} title={t.newTitle}>
      <SupplierForm apiPath={purchasingApi.suppliers(companyId)} lookupApi={vatRegistrantsApi(companyId)} suppliersPath={purchasingPage(companyId, 'suppliers')} listHref={path} canManage />
    </FormPage>
  )
}
