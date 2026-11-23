import type { Metadata } from 'next'
import { texts } from '@/config'
import { WhtTypeForm } from '@/features/finance/Forms'
import { financeApi, financePage, perms } from '@/features/finance/types'
import { FormPage, NoPermission } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.finance.whtTypes.newTitle }

export default async function Page({ params }: { params: Promise<{ companyId: string }> }) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.settingsManage)) return <NoPermission />
  const path = financePage(companyId, 'wht-types')
  const t = texts.finance.whtTypes
  return (
    <FormPage crumb={{ app: texts.apps.finance.name, label: t.heading, href: path }} title={t.newTitle}>
      <WhtTypeForm apiPath={financeApi.whtTypes(companyId)} listHref={path} canManage />
    </FormPage>
  )
}
