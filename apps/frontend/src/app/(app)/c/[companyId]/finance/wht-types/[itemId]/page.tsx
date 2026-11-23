import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { WhtTypeForm } from '@/features/finance/Forms'
import { financeApi, financePage, MASTER_QUERY_KEYS, perms, type WhtType } from '@/features/finance/types'
import { FormPage, loadItem } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.finance.whtTypes.pageTitle }

type Props = { params: Promise<{ companyId: string; itemId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId, itemId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.finance.whtTypes
  const listHref = listHrefFromBack(financePage(companyId, 'wht-types'), param(await searchParams, 'back'), MASTER_QUERY_KEYS)
  const res = await loadItem<WhtType>(financeApi.whtTypes(companyId), itemId, t.notFound)
  if ('panel' in res) return res.panel
  const w = res.item
  return (
    <FormPage crumb={{ app: texts.apps.finance.name, label: t.heading, href: listHref }} title={format(t.editTitle, { code: w.code })} description={w.name}>
      <WhtTypeForm apiPath={financeApi.whtTypes(companyId)} listHref={listHref} item={w} canManage={can(ctx.permissions, perms.settingsManage)} />
    </FormPage>
  )
}
