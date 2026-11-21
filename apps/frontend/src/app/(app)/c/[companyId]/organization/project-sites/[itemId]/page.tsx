import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { ProjectSiteForm } from '@/features/organization/MasterDataForms'
import { FormPage, loadItem } from '@/features/organization/pages'
import { orgResources, type ProjectSite } from '@/features/organization/resources'
import { today } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.projectSites.pageTitle }

type Props = { params: Promise<{ companyId: string; itemId: string }>; searchParams: Promise<SearchParams> }

/** ดู / แก้ไข — ไม่มีสิทธิ์ manage = อ่านอย่างเดียว; ?back= พากลับรายการพร้อมเงื่อนไขเดิม */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, itemId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const r = orgResources.projectSites
  const listHref = listHrefFromBack(r.page(companyId), param(await searchParams, 'back'), r.queryKeys)
  const res = await loadItem<ProjectSite>(r.api(companyId), itemId, texts.errors.project_site_not_found!)
  if ('panel' in res) return res.panel
  const item = res.item
  return (
    <FormPage crumb={{ label: texts.projectSites.heading, href: listHref }} title={format(texts.projectSites.editTitle, { code: item.code })} description={item.name}>
      <ProjectSiteForm companyId={companyId} apiPath={r.api(companyId)} item={item} canManage={can(ctx.permissions, r.manage)} listHref={listHref} defaultStartDate={today()} />
    </FormPage>
  )
}
