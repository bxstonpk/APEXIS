import type { Metadata } from 'next'
import Link from 'next/link'
import { SelectField, type Column } from '@/components'
import { texts } from '@/config'
import { ListHeader, ListToolbar, ListView, RefText, rowHref, rowLinkClass, StatusBadge } from '@/features/lists/ListParts'
import { RefFilter } from '@/features/organization/pages'
import { orgResources, statusFilterOptions, type Department } from '@/features/organization/resources'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.departments.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** รายการแผนก — เงื่อนไขอยู่ใน URL; รายการ + ชื่อของตัวกรองอ้างอิง (ถ้ามี) โหลดพร้อมกัน */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const r = orgResources.departments
  const [list, ref0, ref1, ref2] = await Promise.all([
    loadList<Department>(r.api(companyId), sp, r.queryKeys),
    loadRef(`/companies/${companyId}/business-units`, param(sp, 'business_unit_id')),
    loadRef(`/companies/${companyId}/branches`, param(sp, 'branch_id')),
    loadRef(r.api(companyId), param(sp, 'parent_id')),
  ])
  const f = texts.departments.fields
  const path = r.page(companyId)
  const columns: Column<Department>[] = [
    { key: 'code', header: f.code, sortKey: 'code', render: (x) => <Link href={rowHref(path, x.id, list.query)} className={rowLinkClass}>{x.code}</Link> },
    { key: 'name', header: f.name, sortKey: 'name', render: (x) => x.name },
    { key: 'parent', header: f.parent, hideOnMobile: true, render: (x) => <RefText value={x.parent} /> },
    { key: 'unit', header: f.businessUnit, hideOnMobile: true, render: (x) => <RefText value={x.business_unit} /> },
    { key: 'branch', header: f.branch, hideOnMobile: true, render: (x) => <RefText value={x.branch} /> },
    { key: 'status', header: f.status, render: (x) => <StatusBadge status={x.status} /> },
  ]

  return (
    <>
      <ListHeader title={texts.departments.heading} createHref={can(ctx.permissions, r.manage) && `${path}/new`} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusFilterOptions()} />
        <RefFilter label={f.businessUnit} name="business_unit_id" source={`/companies/${companyId}/business-units`} value={ref0} />
        <RefFilter label={f.branch} name="branch_id" source={`/companies/${companyId}/branches`} value={ref1} />
        <RefFilter label={f.parent} name="parent_id" source={r.api(companyId)} value={ref2} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(x) => x.id} sort={param(sp, 'sort') || r.defaultSort} createHref={can(ctx.permissions, r.manage) && `${path}/new`} />
    </>
  )
}
