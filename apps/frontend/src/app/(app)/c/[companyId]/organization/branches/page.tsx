import type { Metadata } from 'next'
import Link from 'next/link'
import { Badge, SelectField, type Column } from '@/components'
import { texts } from '@/config'
import type { Branch } from '@/features/branches/types'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass, StatusBadge } from '@/features/lists/ListParts'
import { orgResources, statusFilterOptions } from '@/features/organization/resources'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.branches.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/**
 * รายการสาขา — เงื่อนไขทั้งหมดอยู่ใน URL (?q=&status=&sort=&page=) แล้วส่งต่อให้ API ตรงๆ
 * คิวรี backend 1 ครั้ง (GET /branches — ค้นหา / เรียง / แบ่งหน้าใน DB); สิทธิ์มาจาก context ที่ AppShell โหลดไว้แล้ว (cache)
 */
export default async function BranchesPage({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null // AppShell แสดงข้อความแล้ว

  const r = orgResources.branches
  const list = await loadList<Branch>(r.api(companyId), sp, r.queryKeys)
  const t = texts.branches
  const path = r.page(companyId)

  const columns: Column<Branch>[] = [
    {
      key: 'code',
      header: t.columns.code,
      sortKey: 'code',
      render: (b) => (
        <Link href={rowHref(path, b.id, list.query)} className={rowLinkClass}>
          {b.code}
        </Link>
      ),
    },
    {
      key: 'name',
      header: t.columns.name,
      sortKey: 'name',
      render: (b) => (
        <span className="flex flex-wrap items-center gap-2">
          {b.name}
          {b.is_head_office && <Badge tone="info">{t.headOffice}</Badge>}
        </span>
      ),
    },
    { key: 'tax', header: t.columns.taxBranchNo, sortKey: 'tax_branch_no', render: (b) => b.tax_branch_no },
    { key: 'phone', header: t.columns.phone, hideOnMobile: true, render: (b) => b.phone },
    { key: 'province', header: t.columns.province, hideOnMobile: true, render: (b) => b.address.province },
    { key: 'status', header: t.columns.status, render: (b) => <StatusBadge status={b.status} /> },
  ]

  return (
    <>
      <ListHeader title={t.heading} createHref={can(ctx.permissions, r.manage) && `${path}/new`} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusFilterOptions()} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(b) => b.id} sort={param(sp, 'sort') || r.defaultSort} createHref={can(ctx.permissions, r.manage) && `${path}/new`} />
    </>
  )
}
