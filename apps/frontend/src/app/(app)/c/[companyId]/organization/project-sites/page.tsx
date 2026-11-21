import type { Metadata } from 'next'
import Link from 'next/link'
import { SelectField, TextField, type Column } from '@/components'
import { texts } from '@/config'
import { ListHeader, ListToolbar, ListView, RefText, rowHref, rowLinkClass, StatusBadge } from '@/features/lists/ListParts'
import { RefFilter } from '@/features/organization/pages'
import { orgResources, statusFilterOptions, type ProjectSite } from '@/features/organization/resources'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.projectSites.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** รายการโครงการ / ไซต์งาน — เงื่อนไขอยู่ใน URL; รายการ + ชื่อของตัวกรองอ้างอิง (ถ้ามี) โหลดพร้อมกัน */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const r = orgResources.projectSites
  const [list, ref0, ref1] = await Promise.all([
    loadList<ProjectSite>(r.api(companyId), sp, r.queryKeys),
    loadRef(`/companies/${companyId}/branches`, param(sp, 'branch_id')),
    loadRef(`/companies/${companyId}/cost-centers`, param(sp, 'cost_center_id')),
  ])
  const f = texts.projectSites.fields
  const path = r.page(companyId)
  const columns: Column<ProjectSite>[] = [
    { key: 'code', header: f.code, sortKey: 'code', render: (x) => <Link href={rowHref(path, x.id, list.query)} className={rowLinkClass}>{x.code}</Link> },
    { key: 'name', header: f.name, sortKey: 'name', render: (x) => x.name },
    { key: 'branch', header: f.branch, hideOnMobile: true, render: (x) => <RefText value={x.branch} /> },
    { key: 'start', header: f.startDate, sortKey: 'start_date', render: (x) => formatDate(x.start_date) },
    { key: 'end', header: f.endDate, hideOnMobile: true, render: (x) => formatDate(x.end_date) || '–' },
    { key: 'status', header: f.status, render: (x) => <StatusBadge status={x.status} labels={texts.projectSites.statuses} /> },
  ]

  return (
    <>
      <ListHeader title={texts.projectSites.heading} createHref={can(ctx.permissions, r.manage) && `${path}/new`} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusFilterOptions(texts.projectSites.statuses)} />
        <RefFilter label={f.branch} name="branch_id" source={`/companies/${companyId}/branches`} value={ref0} />
        <RefFilter label={f.costCenter} name="cost_center_id" source={`/companies/${companyId}/cost-centers`} value={ref1} />
        <TextField type="date" name="start_date" label={texts.common.dateFrom} title={texts.common.dateFrom} defaultValue={param(sp, 'start_date')} className="w-full sm:w-40" />
        <TextField type="date" name="end_date" label={texts.common.dateTo} title={texts.common.dateTo} defaultValue={param(sp, 'end_date')} className="w-full sm:w-40" />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(x) => x.id} sort={param(sp, 'sort') || r.defaultSort} createHref={can(ctx.permissions, r.manage) && `${path}/new`} />
    </>
  )
}
