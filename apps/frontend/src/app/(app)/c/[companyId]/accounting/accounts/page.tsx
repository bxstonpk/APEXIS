import type { Metadata } from 'next'
import Link from 'next/link'
import { Alert, Badge, SelectField, type Column } from '@/components'
import { texts } from '@/config'
import { AccountTree } from '@/features/accounting/AccountTree'
import { DefaultChartPanel } from '@/features/accounting/DefaultChartPanel'
import { ACCOUNT_QUERY_KEYS, ACCOUNT_TYPES, ledgerApi, ledgerPage, perms, type Account } from '@/features/accounting/types'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass, StatusBadge } from '@/features/lists/ListParts'
import { statusFilterOptions } from '@/features/organization/resources'
import { errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { loadList } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.accounts.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/**
 * ผังบัญชี
 * - ไม่มีเงื่อนไข = ต้นไม้ทั้งผัง (GET /accounts/tree — 1 คิวรี, ผังบัญชีหลักร้อยบัญชีจึงส่งทั้งชุด)
 * - มีคำค้น / ตัวกรอง = รายการแบบแบน กรอง เรียง และแบ่งหน้าใน DB (GET /accounts)
 * - ยังไม่มีบัญชี = ปุ่มสร้างผังบัญชีมาตรฐาน
 */
export default async function AccountsPage({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.accounts
  const path = ledgerPage.accounts(companyId)
  const createHref = can(ctx.permissions, perms.accountManage) && `${path}/new`
  const filtered = ['q', 'type', 'status'].some((k) => param(sp, k))

  const toolbar = (
    <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
      <SelectField
        name="type"
        label={t.fields.type}
        defaultValue={param(sp, 'type')}
        options={[{ value: '', label: texts.common.all }, ...ACCOUNT_TYPES.map((x) => ({ value: x, label: t.types[x] ?? x }))]}
      />
      <SelectField name="status" label={texts.common.status} defaultValue={param(sp, 'status')} options={statusFilterOptions()} />
    </ListToolbar>
  )

  if (filtered) {
    const list = await loadList<Account>(ledgerApi.accounts(companyId), sp, ACCOUNT_QUERY_KEYS)
    const columns: Column<Account>[] = [
      {
        key: 'code',
        header: t.columns.code,
        sortKey: 'code',
        render: (a) => (
          <Link href={rowHref(path, a.id, list.query)} className={`${rowLinkClass} font-mono text-[13px]`}>
            {a.code}
          </Link>
        ),
      },
      {
        key: 'name',
        header: t.columns.name,
        sortKey: 'name',
        render: (a) => (
          <span className="block min-w-0">
            <span className={a.postable ? undefined : 'font-semibold'}>{a.name}</span>
            {a.name_en && <span className="block truncate text-[13px] text-ink-muted">{a.name_en}</span>}
          </span>
        ),
      },
      { key: 'parent', header: t.fields.parent, hideOnMobile: true, render: (a) => (a.parent ? <span className="text-[13px] text-ink-muted">{a.parent.code} {a.parent.name}</span> : '–') },
      { key: 'type', header: t.columns.type, sortKey: 'type', render: (a) => (<span className="inline-flex flex-wrap items-center gap-1.5">{t.types[a.type] ?? a.type}{!a.postable && <Badge>{t.group}</Badge>}</span>) },
      { key: 'status', header: t.columns.status, render: (a) => <StatusBadge status={a.status} /> },
    ]
    return (
      <>
        <ListHeader title={t.heading} description={t.description} createHref={createHref} count={list.error ? undefined : list.meta.total} />
        {toolbar}
        <p className="mb-3 text-[13px]">
          <Link href={path} className="font-medium text-primary hover:underline">
            ← {t.backToTree}
          </Link>
        </p>
        <ListView list={list} path={path} columns={columns} rowKey={(a) => a.id} sort={param(sp, 'sort') || 'code'} createHref={createHref} />
      </>
    )
  }

  const tree = await backendGet<Account[]>(ledgerApi.accountTree(companyId)).then(
    (r) => r.data,
    (err: unknown) => err,
  )
  if (!Array.isArray(tree)) {
    return (
      <>
        <ListHeader title={t.heading} description={t.description} createHref={createHref} />
        <Alert tone="error">{errorMessage(tree)}</Alert>
      </>
    )
  }
  return (
    <>
      <ListHeader title={t.heading} description={t.description} createHref={tree.length > 0 && createHref} count={tree.length} />
      {tree.length === 0 ? (
        <DefaultChartPanel apiPath={ledgerApi.accountDefaults(companyId)} createHref={createHref} />
      ) : (
        <>
          {toolbar}
          <AccountTree accounts={tree} basePath={path} />
        </>
      )}
    </>
  )
}
