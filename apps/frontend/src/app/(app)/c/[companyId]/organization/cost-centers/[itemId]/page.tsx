import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { CostCenterForm } from '@/features/organization/MasterDataForms'
import { FormPage, loadItem } from '@/features/organization/pages'
import { orgResources, type CostCenter } from '@/features/organization/resources'
import { today } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.costCenters.pageTitle }

type Props = { params: Promise<{ companyId: string; itemId: string }>; searchParams: Promise<SearchParams> }

/** ดู / แก้ไข — ไม่มีสิทธิ์ manage = อ่านอย่างเดียว; ?back= พากลับรายการพร้อมเงื่อนไขเดิม */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, itemId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const r = orgResources.costCenters
  const listHref = listHrefFromBack(r.page(companyId), param(await searchParams, 'back'), r.queryKeys)
  const res = await loadItem<CostCenter>(r.api(companyId), itemId, texts.errors.cost_center_not_found!)
  if ('panel' in res) return res.panel
  const item = res.item
  return (
    <FormPage crumb={{ label: texts.costCenters.heading, href: listHref }} title={format(texts.costCenters.editTitle, { code: item.code })} description={item.name}>
      <CostCenterForm companyId={companyId} apiPath={r.api(companyId)} item={item} canManage={can(ctx.permissions, r.manage)} listHref={listHref} defaultValidFrom={today()} />
    </FormPage>
  )
}
