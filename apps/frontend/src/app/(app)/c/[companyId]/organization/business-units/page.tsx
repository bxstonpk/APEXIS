import type { Metadata } from 'next'
import Link from 'next/link'
import { SelectField, type Column } from '@/components'
import { texts } from '@/config'
import { ListHeader, ListToolbar, ListView, RefText, rowHref, rowLinkClass, StatusBadge } from '@/features/lists/ListParts'
import { RefFilter } from '@/features/organization/pages'
import { orgResources, statusFilterOptions, type BusinessUnit } from '@/features/organization/resources'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.businessUnits.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** รายการหน่วยธุรกิจ — เงื่อนไขอยู่ใน URL; รายการ + ชื่อของตัวกรองอ้างอิง (ถ้ามี) โหลดพร้อมกัน */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const r = orgResources.businessUnits
  const [list, ref0] = await Promise.all([
    loadList<BusinessUnit>(r.api(companyId), sp, r.queryKeys),
    loadRef(r.api(companyId), param(sp, 'parent_id')),
  ])
  const f = texts.businessUnits.fields
  const path = r.page(companyId)
  const columns: Column<BusinessUnit>[] = [
    { key: 'code', header: f.code, sortKey: 'code', render: (x) => <Link href={rowHref(path, x.id, list.query)} className={rowLinkClass}>{x.code}</Link> },
    { key: 'name', header: f.name, sortKey: 'name', render: (x) => x.name },
    { key: 'parent', header: f.parent, render: (x) => <RefText value={x.parent} /> },
    { key: 'status', header: f.status, render: (x) => <StatusBadge status={x.status} /> },
  ]

  return (
    <>
      <ListHeader title={texts.businessUnits.heading} createHref={can(ctx.permissions, r.manage) && `${path}/new`} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusFilterOptions()} />
        <RefFilter label={f.parent} name="parent_id" source={r.api(companyId)} value={ref0} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(x) => x.id} sort={param(sp, 'sort') || r.defaultSort} createHref={can(ctx.permissions, r.manage) && `${path}/new`} />
    </>
  )
}
