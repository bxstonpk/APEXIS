import type { Metadata } from 'next'
import Link from 'next/link'
import { SelectField, TextField, type Column } from '@/components'
import { texts } from '@/config'
import { ListHeader, ListToolbar, ListView, RefText, rowHref, rowLinkClass, StatusBadge } from '@/features/lists/ListParts'
import { RefFilter } from '@/features/organization/pages'
import { orgResources, statusFilterOptions, type CostCenter } from '@/features/organization/resources'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.costCenters.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** รายการศูนย์ต้นทุน — เงื่อนไขอยู่ใน URL; รายการ + ชื่อของตัวกรองอ้างอิง (ถ้ามี) โหลดพร้อมกัน */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const r = orgResources.costCenters
  const [list, ref0] = await Promise.all([
    loadList<CostCenter>(r.api(companyId), sp, r.queryKeys),
    loadRef(`/companies/${companyId}/departments`, param(sp, 'department_id')),
  ])
  const f = texts.costCenters.fields
  const path = r.page(companyId)
  const columns: Column<CostCenter>[] = [
    { key: 'code', header: f.code, sortKey: 'code', render: (x) => <Link href={rowHref(path, x.id, list.query)} className={rowLinkClass}>{x.code}</Link> },
    { key: 'name', header: f.name, sortKey: 'name', render: (x) => x.name },
    { key: 'department', header: f.department, hideOnMobile: true, render: (x) => <RefText value={x.department} /> },
    { key: 'from', header: f.validFrom, sortKey: 'valid_from', render: (x) => formatDate(x.valid_from) },
    { key: 'to', header: f.validTo, hideOnMobile: true, render: (x) => formatDate(x.valid_to) || '–' },
    { key: 'status', header: f.status, render: (x) => <StatusBadge status={x.status} /> },
  ]

  return (
    <>
      <ListHeader title={texts.costCenters.heading} createHref={can(ctx.permissions, r.manage) && `${path}/new`} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusFilterOptions()} />
        <RefFilter label={f.department} name="department_id" source={`/companies/${companyId}/departments`} value={ref0} />
        <TextField type="date" name="active_on" label={texts.costCenters.activeOn} title={texts.costCenters.activeOn} defaultValue={param(sp, 'active_on')} className="w-full sm:w-40" />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(x) => x.id} sort={param(sp, 'sort') || r.defaultSort} createHref={can(ctx.permissions, r.manage) && `${path}/new`} />
    </>
  )
}
