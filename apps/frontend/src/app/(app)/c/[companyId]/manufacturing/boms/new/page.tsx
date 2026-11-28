import type { Metadata } from 'next'
import { texts } from '@/config'
import { BomForm } from '@/features/manufacturing/BomForm'
import { mfgApi, mfgPage, mfgPerms } from '@/features/manufacturing/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.manufacturing.boms.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, mfgPerms.bomManage)) return <NoPermission />
  const t = texts.manufacturing.boms
  const path = mfgPage(companyId, 'boms')
  return (
    <FormPage crumb={{ app: texts.apps.manufacturing.name, label: t.heading, href: path }} title={t.newTitle}>
      <BomForm apiPath={mfgApi.boms(companyId)} listHref={path} itemsApi={mfgApi.items(companyId)} accountsApi={mfgApi.accounts(companyId)} canManage />
    </FormPage>
  )
}
