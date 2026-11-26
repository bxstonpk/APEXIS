import type { Metadata } from 'next'
import { format, texts } from '@/config'
import { BomForm } from '@/features/manufacturing/BomForm'
import { BOM_QUERY_KEYS, mfgApi, mfgPage, mfgPerms, type Bom } from '@/features/manufacturing/types'
import { FormPage, loadItem } from '@/features/organization/pages'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.manufacturing.boms.pageTitle }

type Props = { params: Promise<{ companyId: string; bomId: string }>; searchParams: Promise<SearchParams> }

/** สูตรการผลิต — แก้ไขได้เมื่อมีสิทธิ์จัดการ (ไม่มีสิทธิ์ = อ่านอย่างเดียว) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, bomId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.manufacturing.boms
  const listHref = listHrefFromBack(mfgPage(companyId, 'boms'), param(await searchParams, 'back'), BOM_QUERY_KEYS)
  const res = await loadItem<Bom>(mfgApi.boms(companyId), bomId, t.notFound)
  if ('panel' in res) return res.panel
  const b = res.item
  return (
    <FormPage crumb={{ app: texts.apps.manufacturing.name, label: t.heading, href: listHref }} title={format(t.editTitle, { code: b.code })} description={b.name}>
      <BomForm
        apiPath={mfgApi.boms(companyId)}
        listHref={listHref}
        itemsApi={mfgApi.items(companyId)}
        accountsApi={mfgApi.accounts(companyId)}
        canManage={can(ctx.permissions, mfgPerms.bomManage)}
        bom={b}
      />
    </FormPage>
  )
}
