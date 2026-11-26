import { Alert, Badge, SelectField, TextField, type Column } from '@/components'
import { format, texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { ListHeader, ListToolbar, ListView } from '@/features/lists/ListParts'
import { FormPage, loadItem, NoPermission, RefFilter } from '@/features/organization/pages'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack, loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'
import { DocActions, type DocAction } from './Actions'
import { BillingNoteForm } from './BillingNoteForm'
import { loadLetterhead, noCell, SalesStatus } from './pages'
import { PrintDocument, printDate } from './PrintDocument'
import { PrintToolbar } from './PrintToolbar'
import { BILLING_QUERY_KEYS, perms, SALE_MENU, salesApi, salesPage, type BillingNote } from './types'
import { dateText, EntryLink, Facts } from './Views'

/** ใบวางบิล (server) — รวมใบค้างชำระของลูกค้า, ไม่มีผลทางบัญชี; รับชำระตามใบวางบิลที่แอปการเงิน */
export async function BillingNoteListPage({ companyId, sp }: { companyId: string; sp: SearchParams }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales
  const b = t.billingNotes
  const path = salesPage(companyId, 'billing-notes')
  const createHref = can(ctx.permissions, perms.billingNoteManage) && `${path}/new`
  const [list, customer] = await Promise.all([loadList<BillingNote>(salesApi.billingNotes(companyId), sp, BILLING_QUERY_KEYS), loadRef(salesApi.customers(companyId), param(sp, 'customer_id'))])
  const columns: Column<BillingNote>[] = [
    { key: 'date', header: t.columns.date, sortKey: 'date', render: (d) => <span className="whitespace-nowrap">{formatDate(d.date)}</span> },
    { key: 'no', header: t.columns.no, sortKey: 'document_no', render: (d) => noCell(path, d.id, d.document_no, list.query, '') },
    { key: 'customer', header: t.columns.customer, render: (d) => <span className="block truncate">{d.customer.name}</span> },
    { key: 'due', header: b.fields.dueDate, hideOnMobile: true, render: (d) => <span className="whitespace-nowrap">{formatDate(d.due_date)}</span> },
    { key: 'total', header: b.fields.total, align: 'right', hideOnMobile: true, render: (d) => <span className="tabular-nums">{formatAmount(d.total)}</span> },
    { key: 'outstanding', header: b.fields.outstanding, align: 'right', render: (d) => <span className="font-medium tabular-nums">{d.status === 'cancelled' ? '–' : formatAmount(d.outstanding, { dashZero: true })}</span> },
    { key: 'status', header: t.columns.status, render: (d) => <SalesStatus kind="billing" status={d.status} /> },
  ]
  return (
    <>
      <ListHeader title={b.heading} createHref={createHref} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField
          name="status"
          label={texts.common.status}
          defaultValue={param(sp, 'status')}
          options={[{ value: '', label: texts.common.all }, ...Object.entries(t.statuses.billing).map(([value, label]) => ({ value, label }))]}
        />
        <RefFilter label={t.fields.customer} name="customer_id" source={salesApi.customers(companyId)} value={customer} />
        <TextField name="start_date" type="date" label={texts.common.dateFrom} defaultValue={param(sp, 'start_date')} />
        <TextField name="end_date" type="date" label={texts.common.dateTo} defaultValue={param(sp, 'end_date')} />
      </ListToolbar>
      <ListView list={list} path={path} columns={columns} rowKey={(d) => d.id} sort={param(sp, 'sort') || '-date'} createHref={createHref} />
    </>
  )
}

export async function BillingNoteNewPage({ companyId }: { companyId: string }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.billingNoteManage)) return <NoPermission />
  const b = texts.sales.billingNotes
  const path = salesPage(companyId, 'billing-notes')
  return (
    <FormPage crumb={{ app: texts.apps.sales.name, label: b.heading, href: path }} title={b.newTitle}>
      <BillingNoteForm
        apiPath={salesApi.billingNotes(companyId)}
        customersApi={salesApi.customers(companyId)}
        invoicesApi={salesApi.invoices(companyId)}
        serviceInvoicesApi={salesApi.serviceInvoices(companyId)}
        docsPath={path}
        listHref={path}
      />
    </FormPage>
  )
}

export async function BillingNoteViewPage({ companyId, docId, sp }: { companyId: string; docId: string; sp: SearchParams }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales
  const b = t.billingNotes
  const path = salesPage(companyId, 'billing-notes')
  const listHref = listHrefFromBack(path, param(sp, 'back'), BILLING_QUERY_KEYS)
  const res = await loadItem<BillingNote>(salesApi.billingNotes(companyId), docId, b.notFound)
  if ('panel' in res) return res.panel
  const d = res.item
  const open = d.status === 'open'
  const has = (p: string) => can(ctx.permissions, p)
  const actions: DocAction[] = []
  if (open && has(perms.billingNoteManage)) actions.push({ kind: 'patch', status: 'cancelled', label: b.cancel, confirm: format(b.confirmCancel, { no: d.document_no }), danger: true })
  actions.push({ kind: 'link', href: `${path}/${d.id}/print`, label: t.actions.print, icon: 'print' })
  const th = 'px-3 py-2.5 text-left text-[13px] font-medium text-ink-muted'
  const num = 'px-3 py-2.5 text-right tabular-nums'
  return (
    <FormPage crumb={{ app: texts.apps.sales.name, label: b.heading, href: listHref }} title={d.document_no} description={d.customer.name} aside={<SalesStatus kind="billing" status={d.status} />}>
      <div className="flex flex-col gap-6">
        <DocActions apiPath={`${salesApi.billingNotes(companyId)}/${d.id}`} listHref={listHref} docNo={d.document_no} actions={actions} />
        <Facts
          items={[
            [t.fields.customer, `${d.customer.code} ${d.customer.name}`],
            [t.fields.date, dateText(d.date)],
            [b.fields.dueDate, dateText(d.due_date)],
            [b.fields.total, formatAmount(d.total)],
            [b.fields.outstanding, formatAmount(d.outstanding)],
            [t.fields.description, d.description],
          ]}
        />
        <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
          <table className="w-full min-w-[40rem] border-collapse text-sm">
            <thead>
              <tr className="border-b border-line bg-subtle">
                <th scope="col" className={th}>
                  {t.columns.no}
                </th>
                <th scope="col" className={th}>
                  {t.columns.date}
                </th>
                <th scope="col" className={th}>
                  {t.columns.due}
                </th>
                <th scope="col" className={`${th} text-right`}>
                  {t.columns.total}
                </th>
                <th scope="col" className={`${th} text-right`}>
                  {t.columns.outstanding}
                </th>
              </tr>
            </thead>
            <tbody>
              {d.invoices.map((i) => (
                <tr key={i.id} className="border-b border-line last:border-b-0">
                  <td className="px-3 py-2.5">
                    <span className="font-mono text-[13px]">
                      <EntryLink href={`${salesPage(companyId, SALE_MENU[i.kind])}/${i.id}`} no={i.document_no} />
                    </span>
                    <span className="ml-2">
                      <Badge tone={i.kind === 'service_invoice' ? 'info' : 'neutral'}>{t.kinds[i.kind] ?? i.kind}</Badge>
                    </span>
                  </td>
                  <td className="px-3 py-2.5 whitespace-nowrap">{dateText(i.date)}</td>
                  <td className="px-3 py-2.5 whitespace-nowrap">{dateText(i.due_date)}</td>
                  <td className={num}>{formatAmount(i.gross)}</td>
                  <td className={`${num} font-medium`}>{formatAmount(i.outstanding, { dashZero: true })}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </FormPage>
  )
}

/** พิมพ์ใบวางบิล A4 — รายการใบแจ้งหนี้ + ยอดค้าง, นัดชำระ, ลงนามผู้วางบิล / ผู้รับวางบิล */
export async function BillingNotePrintPage({ companyId, docId }: { companyId: string; docId: string }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const p = texts.sales.print
  const [res, letterhead] = await Promise.all([loadItem<BillingNote>(salesApi.billingNotes(companyId), docId, texts.sales.billingNotes.notFound), loadLetterhead(companyId)])
  if ('panel' in res) return res.panel
  const d = res.item
  if (!letterhead) return <Alert tone="error">{p.unavailable}</Alert>
  return (
    <>
      <PrintToolbar backHref={`${salesPage(companyId, 'billing-notes')}/${d.id}`} />
      {d.status === 'cancelled' && (
        <div className="mx-auto mb-4 max-w-[210mm] print:hidden">
          <Alert tone="error">{p.cancelled}</Alert>
        </div>
      )}
      <PrintDocument
        letterhead={letterhead}
        title={p.billingNote}
        docNo={d.document_no}
        info={[
          [p.date, printDate(d.date)],
          [p.appointment, printDate(d.due_date)],
        ]}
        party={{ label: p.customer, name: d.customer.name }}
        lines={d.invoices.map((i) => ({
          description: `${texts.sales.kinds[i.kind] ?? ''} ${i.document_no}`.trim(),
          detail: `${p.date} ${printDate(i.date)} · ${p.dueDate} ${printDate(i.due_date)}`,
          amount: i.outstanding,
        }))}
        totals={[{ label: p.grand, value: d.outstanding, strong: true }]}
        amountInWords={d.outstanding}
        note={d.description || undefined}
        signatures={[p.billingIssuer, p.billingSign]}
      />
    </>
  )
}
