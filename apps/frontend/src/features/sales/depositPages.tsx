import type { ReactNode } from 'react'
import { Alert, type Column } from '@/components'
import { format, texts } from '@/config'
import { companyPath } from '@/config/apps'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { ListHeader, ListView } from '@/features/lists/ListParts'
import { FormPage, loadItem, NoPermission } from '@/features/organization/pages'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack, loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'
import { DocActions, type DocAction } from './Actions'
import { DepositForm } from './DepositForm'
import { DocFilters, loadLetterhead, loadVat, noCell, PaymentBadge, SalesStatus } from './pages'
import { branchLabel, PrintDocument, printDate } from './PrintDocument'
import { PrintToolbar } from './PrintToolbar'
import { DOC_QUERY_KEYS, perms, salesApi, salesPage, type Deposit, type Order } from './types'
import { dateText, EntryLink, Facts } from './Views'

/**
 * ใบรับเงินมัดจำ / ใบกำกับภาษี (server) — ภาษีขายเกิดวันที่เอกสาร, ลงบัญชีลูกหนี้ / มัดจำรับ / ภาษีขาย
 * รับเงินยืนยันที่แอปการเงิน; รับครบแล้วจึงหักในเอกสารขายได้ (คงเหลือหักได้ = ก่อน VAT)
 */
export async function DepositListPage({ companyId, sp }: { companyId: string; sp: SearchParams }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales
  const path = salesPage(companyId, 'deposits')
  const createHref = can(ctx.permissions, perms.invoicePost) && `${path}/new`
  const [list, customer] = await Promise.all([loadList<Deposit>(salesApi.deposits(companyId), sp, DOC_QUERY_KEYS), loadRef(salesApi.customers(companyId), param(sp, 'customer_id'))])
  const columns: Column<Deposit>[] = [
    { key: 'date', header: t.columns.date, sortKey: 'date', render: (d) => <span className="whitespace-nowrap">{formatDate(d.date)}</span> },
    { key: 'no', header: t.columns.no, sortKey: 'document_no', render: (d) => noCell(path, d.id, d.document_no, list.query, '') },
    {
      key: 'customer',
      header: t.columns.customer,
      render: (d) => (
        <span className="block min-w-0">
          <span className="block truncate">{d.customer.name}</span>
          {d.description && <span className="block truncate text-[13px] text-ink-muted">{d.description}</span>}
        </span>
      ),
    },
    { key: 'gross', header: t.columns.total, align: 'right', hideOnMobile: true, render: (d) => <span className="tabular-nums">{formatAmount(d.gross)}</span> },
    { key: 'remaining', header: t.columns.remaining, align: 'right', render: (d) => <span className="font-medium tabular-nums">{d.status === 'cancelled' ? '–' : formatAmount(d.remaining_amount, { dashZero: true })}</span> },
    { key: 'paid', header: t.fields.received, hideOnMobile: true, render: (d) => <PaymentBadge outstanding={d.outstanding} status={d.status} /> },
    { key: 'status', header: t.columns.status, render: (d) => <SalesStatus kind="posted" status={d.status} /> },
  ]
  return (
    <>
      <ListHeader title={t.deposits.heading} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <DocFilters sp={sp} companyId={companyId} kind="posted" customer={customer} />
      <ListView list={list} path={path} columns={columns} rowKey={(d) => d.id} sort={param(sp, 'sort') || '-date'} createHref={createHref} />
    </>
  )
}

export async function DepositNewPage({ companyId, orderId }: { companyId: string; orderId: string }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.invoicePost)) return <NoPermission />
  const t = texts.sales
  const path = salesPage(companyId, 'deposits')
  let order: Order | undefined
  if (orderId) {
    const res = await loadItem<Order>(salesApi.orders(companyId), orderId, t.orders.notFound)
    if ('panel' in res) return res.panel
    order = res.item
  }
  const vat = order ? null : await loadVat(companyId)
  return (
    <FormPage crumb={{ app: texts.apps.sales.name, label: t.deposits.heading, href: path }} title={t.deposits.newTitle} description={order ? `${order.document_no ?? ''} · ${order.customer.name}` : undefined}>
      <DepositForm
        apiPath={salesApi.deposits(companyId)}
        customersApi={salesApi.customers(companyId)}
        docsPath={path}
        listHref={order ? `${salesPage(companyId, 'orders')}/${order.id}` : path}
        vat={vat}
        order={order}
      />
    </FormPage>
  )
}

export async function DepositViewPage({ companyId, docId, sp }: { companyId: string; docId: string; sp: SearchParams }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales
  const f = t.deposits.fields
  const path = salesPage(companyId, 'deposits')
  const listHref = listHrefFromBack(path, param(sp, 'back'), DOC_QUERY_KEYS)
  const res = await loadItem<Deposit>(salesApi.deposits(companyId), docId, t.deposits.notFound)
  if ('panel' in res) return res.panel
  const d = res.item
  const posted = d.status === 'posted'
  const has = (p: string) => can(ctx.permissions, p)
  const unpaid = posted && toUnits(d.outstanding) > 0n
  const unapplied = toUnits(d.applied_amount) === 0n
  const entryHref = d.entry_id && has('ledger.entry:read') ? `${companyPath(companyId, 'accounting', 'entries')}/${d.entry_id}` : null
  const actions: DocAction[] = []
  if (posted && unpaid && unapplied && has(perms.invoicePost))
    actions.push({ kind: 'patch', status: 'cancelled', label: t.actions.cancelDoc, confirm: t.actions.confirm.cancelDoc, danger: true })
  actions.push({ kind: 'link', href: `${path}/${d.id}/print`, label: t.actions.print, icon: 'print' })
  const facts: [string, ReactNode][] = [
    [t.fields.customer, `${d.customer.code} ${d.bill_to.name || d.customer.name}`],
    [t.fields.customerBranch, branchLabel(d.bill_to.branch_no)],
    [t.fields.date, dateText(d.date)],
    [t.columns.order, d.order ? <EntryLink key="o" href={`${salesPage(companyId, 'orders')}/${d.order.id}`} no={d.order.order_no} /> : ''],
    [t.fields.entry, <EntryLink key="e" href={entryHref} no={d.entry_no} />],
    [f.description, d.description],
    [t.fields.received, formatAmount(d.received_amount)],
    [t.columns.outstanding, posted ? formatAmount(d.outstanding) : ''],
  ]
  return (
    <FormPage
      crumb={{ app: texts.apps.sales.name, label: t.deposits.heading, href: listHref }}
      title={d.document_no}
      description={d.customer.name}
      aside={
        <span className="flex flex-wrap items-center gap-2">
          <PaymentBadge outstanding={d.outstanding} status={d.status} />
          <SalesStatus kind="posted" status={d.status} />
        </span>
      }
    >
      <div className="flex flex-col gap-6">
        <DocActions apiPath={`${salesApi.deposits(companyId)}/${d.id}`} listHref={listHref} docNo={d.document_no} actions={actions} />
        <Facts items={facts} />
        <dl className="grid gap-3 rounded-(--radius-panel) border border-line bg-surface p-5 text-sm shadow-card sm:grid-cols-3 lg:grid-cols-5">
          {(
            [
              [f.base, d.base],
              [format(t.totals.vat, { rate: d.vat_rate }), d.vat],
              [f.gross, d.gross],
              [f.applied, d.applied_amount],
              [f.remaining, d.remaining_amount],
            ] as const
          ).map(([label, v]) => (
            <div key={label}>
              <dt className="text-[13px] text-ink-muted">{label}</dt>
              <dd className="mt-0.5 text-[17px] font-semibold tabular-nums">{formatAmount(v)}</dd>
            </div>
          ))}
        </dl>
      </div>
    </FormPage>
  )
}

/** พิมพ์ใบรับเงินมัดจำ / ใบกำกับภาษี A4 (ไม่จด VAT = ใบรับเงินมัดจำ) */
export async function DepositPrintPage({ companyId, docId }: { companyId: string; docId: string }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const p = texts.sales.print
  const [res, letterhead] = await Promise.all([loadItem<Deposit>(salesApi.deposits(companyId), docId, texts.sales.deposits.notFound), loadLetterhead(companyId)])
  if ('panel' in res) return res.panel
  const d = res.item
  if (!letterhead) return <Alert tone="error">{p.unavailable}</Alert>
  const vat = toUnits(d.vat_rate) > 0n
  return (
    <>
      <PrintToolbar backHref={`${salesPage(companyId, 'deposits')}/${d.id}`} />
      {d.status === 'cancelled' && (
        <div className="mx-auto mb-4 max-w-[210mm] print:hidden">
          <Alert tone="error">{p.cancelled}</Alert>
        </div>
      )}
      <PrintDocument
        letterhead={letterhead}
        title={vat ? p.deposit : p.depositNoVat}
        docNo={d.document_no}
        info={[
          [p.date, printDate(d.date)],
          [p.orderNo, d.order?.order_no ?? ''],
        ]}
        party={{ label: p.customer, name: d.bill_to.name || d.customer.name, taxId: d.bill_to.tax_id, branchNo: d.bill_to.branch_no, address: d.bill_to.address }}
        lines={[{ description: p.depositLine, detail: d.description, amount: d.base }]}
        totals={[
          { label: p.subtotal, value: d.base },
          ...(vat ? [{ label: format(p.vat, { rate: d.vat_rate }), value: d.vat }] : []),
          { label: p.grand, value: d.gross, strong: true },
        ]}
        amountInWords={d.gross}
        signatures={[p.receiver, p.approver]}
      />
    </>
  )
}
