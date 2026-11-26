import Link from 'next/link'
import { Badge, SelectField, TextField, type Column } from '@/components'
import { format, texts } from '@/config'
import { companyPath } from '@/config/apps'
import { formatAmount, toUnits } from '@/features/accounting/money'
import { ListHeader, ListToolbar, ListView, rowHref, rowLinkClass } from '@/features/lists/ListParts'
import { FormPage, loadItem, NoPermission, RefFilter } from '@/features/organization/pages'
import { docPerms, purchasingApi, purchasingDocsApi, purchasingPage, type SupplierInvoice } from '@/features/purchasing/types'
import { DocActions, type DocAction } from '@/features/sales/Actions'
import { branchLabel } from '@/features/sales/PrintDocument'
import { perms as salesPerms, salesApi, salesPage, type Invoice } from '@/features/sales/types'
import { dateText, EntryLink, Facts, LinesTable } from '@/features/sales/Views'
import { MessagePanel } from '@/features/shell'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack, loadList, loadRef } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'
import { CreditNoteForm } from './CreditNoteForm'
import {
  CREDIT_QUERY_KEYS,
  type CreditableInvoice,
  type SalesCreditNote,
  type SalesCreditNoteSummary,
  type SupplierCreditNote,
  type SupplierCreditNoteSummary,
} from './types'

/**
 * หน้าใบลดหนี้ (server) — ใช้ร่วมระหว่างแอปขาย (mode=sales: /sales-credit-notes) และจัดซื้อ (mode=purchasing: /supplier-credit-notes)
 * สร้างจากหน้าใบกำกับภาษีเดิม (?invoice_id=), ผ่านรายการทันที, ยกเลิกได้ (กลับรายการสต็อก / บัญชี / ยอดค้าง)
 */
export type CreditMode = 'sales' | 'purchasing'

const cfg = (mode: CreditMode, companyId: string) =>
  mode === 'sales'
    ? {
        app: texts.apps.sales.name,
        api: salesApi.creditNotes(companyId),
        invoicesApi: salesApi.invoices(companyId),
        partyApi: salesApi.customers(companyId),
        partyKey: 'customer_id',
        partyLabel: texts.sales.columns.customer,
        warehousesApi: salesApi.warehouses(companyId),
        path: salesPage(companyId, 'credit-notes'),
        invoicesPath: salesPage(companyId, 'invoices'),
        post: salesPerms.invoicePost,
      }
    : {
        app: texts.apps.purchasing.name,
        api: purchasingDocsApi.creditNotes(companyId),
        invoicesApi: purchasingDocsApi.invoices(companyId),
        partyApi: purchasingApi.suppliers(companyId),
        partyKey: 'supplier_id',
        partyLabel: texts.purchasing.columns.supplier,
        warehousesApi: purchasingDocsApi.warehouses(companyId),
        path: purchasingPage(companyId, 'credit-notes'),
        invoicesPath: purchasingPage(companyId, 'invoices'),
        post: docPerms.invoicePost,
      }

export function CreditStatus({ status }: { status: string }) {
  return (
    <Badge tone={status === 'posted' ? 'success' : 'warning'} dot>
      {texts.sales.statuses.posted[status] ?? status}
    </Badge>
  )
}

export function KindBadge({ kind }: { kind: string }) {
  return <Badge tone={kind === 'return' ? 'info' : 'neutral'}>{texts.creditNotes.kindLabel[kind] ?? kind}</Badge>
}

type Row = SalesCreditNoteSummary | SupplierCreditNoteSummary

export async function CreditNoteList({ mode, companyId, sp }: { mode: CreditMode; companyId: string; sp: SearchParams }) {
  const t = texts.creditNotes
  const page = t[mode]
  const c = cfg(mode, companyId)
  const [list, party] = await Promise.all([loadList<Row>(c.api, sp, CREDIT_QUERY_KEYS), loadRef(c.partyApi, param(sp, c.partyKey))])
  const columns: Column<Row>[] = [
    { key: 'date', header: t.columns.date, sortKey: 'date', render: (d) => <span className="whitespace-nowrap">{formatDate(d.date)}</span> },
    {
      key: 'no',
      header: t.columns.no,
      sortKey: 'document_no',
      render: (d) => (
        <Link href={rowHref(c.path, d.id, list.query)} className={`${rowLinkClass} font-mono text-[13px] whitespace-nowrap`}>
          {d.document_no}
        </Link>
      ),
    },
    {
      key: 'party',
      header: t.columns.party,
      render: (d) => {
        const p = 'customer' in d ? d.customer : d.supplier
        return (
          <span className="block min-w-0">
            <span className="block truncate">{p.name}</span>
            {'supplier_credit_note_no' in d && <span className="block font-mono text-[12px] text-ink-muted">{d.supplier_credit_note_no}</span>}
          </span>
        )
      },
    },
    {
      key: 'invoice',
      header: t.columns.invoice,
      hideOnMobile: true,
      render: (d) => <span className="font-mono text-[13px] whitespace-nowrap">{'supplier_invoice_no' in d.invoice ? d.invoice.supplier_invoice_no : d.invoice.document_no}</span>,
    },
    { key: 'kind', header: t.columns.kind, hideOnMobile: true, render: (d) => <KindBadge kind={d.kind} /> },
    { key: 'total', header: t.columns.total, align: 'right', render: (d) => <span className="font-medium tabular-nums">{formatAmount(d.totals.gross)}</span> },
    { key: 'status', header: t.columns.status, render: (d) => <CreditStatus status={d.status} /> },
  ]
  return (
    <>
      <ListHeader title={page.heading} description={page.description} count={list.error ? undefined : list.meta.total} />
      <ListToolbar q={param(sp, 'q')} keep={{ sort: param(sp, 'sort'), page_size: param(sp, 'page_size') }}>
        <SelectField
          name="status"
          label={texts.common.status}
          defaultValue={param(sp, 'status')}
          options={[{ value: '', label: texts.common.all }, ...Object.entries(texts.sales.statuses.posted).map(([value, label]) => ({ value, label }))]}
        />
        <RefFilter label={c.partyLabel} name={c.partyKey} source={c.partyApi} value={party} />
        <TextField name="start_date" type="date" label={texts.common.dateFrom} defaultValue={param(sp, 'start_date')} />
        <TextField name="end_date" type="date" label={texts.common.dateTo} defaultValue={param(sp, 'end_date')} />
      </ListToolbar>
      <ListView list={list} path={c.path} columns={columns} rowKey={(d) => d.id} sort={param(sp, 'sort') || '-date'} />
    </>
  )
}

/** ใบกำกับเดิม → รูปที่ฟอร์มใช้ */
function creditable(mode: CreditMode, inv: Invoice | SupplierInvoice): CreditableInvoice {
  return {
    id: inv.id,
    no: mode === 'purchasing' ? `${inv.document_no} (${(inv as SupplierInvoice).supplier_invoice_no})` : inv.document_no,
    party: 'supplier_invoice_no' in inv ? inv.supplier.name : inv.customer.name,
    vat_rate: inv.vat_rate,
    price_includes_vat: inv.price_includes_vat,
    outstanding: inv.outstanding,
    lines: inv.lines.map((l) => ({
      line_no: l.line_no,
      item: l.item,
      quantity: l.quantity,
      unit_price: l.unit_price,
      amounts: l.amounts,
      returned_quantity: l.returned_quantity,
      credited_amount: l.credited_amount,
    })),
  }
}

/** หน้าออกใบลดหนี้จาก ?invoice_id= — ใบกำกับต้องผ่านรายการแล้วและยังมียอดค้าง */
export async function CreditNoteNewPage({ mode, companyId, invoiceId }: { mode: CreditMode; companyId: string; invoiceId: string }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const c = cfg(mode, companyId)
  if (!can(ctx.permissions, c.post)) return <NoPermission />
  const t = texts.creditNotes
  const page = t[mode]
  if (!invoiceId) return <MessagePanel title={page.notFound} description={page.pickInvoice} />
  const notFound = mode === 'sales' ? texts.sales.invoices.notFound : texts.purchasing.invoices.notFound
  const res = await loadItem<Invoice | SupplierInvoice>(c.invoicesApi, invoiceId, notFound)
  if ('panel' in res) return res.panel
  const inv = res.item
  const party = 'supplier_invoice_no' in inv ? inv.supplier.name : (inv as Invoice).customer.name
  return (
    <FormPage crumb={{ app: c.app, label: page.heading, href: c.path }} title={page.newTitle} description={`${inv.document_no} · ${party}`}>
      {inv.status === 'posted' && toUnits(inv.outstanding) > 0n ? (
        <CreditNoteForm
          mode={mode}
          invoice={creditable(mode, inv)}
          apiPath={c.api}
          warehousesApi={c.warehousesApi}
          docsPath={c.path}
          invoiceHref={`${c.invoicesPath}/${inv.id}`}
        />
      ) : (
        <MessagePanel title={t.form.overOutstanding.replace(' ({amount})', '')} description={page.pickInvoice} />
      )}
    </FormPage>
  )
}

/** ดูใบลดหนี้ — ข้อมูลอ้างอิงใบกำกับเดิม, เหตุผล, มูลค่าเดิม / ที่ถูกต้อง / ผลต่าง (ขาย), รายการ, ยกเลิก / พิมพ์ */
export async function CreditNoteViewPage({ mode, companyId, docId, sp }: { mode: CreditMode; companyId: string; docId: string; sp: SearchParams }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.creditNotes
  const f = t.fields
  const page = t[mode]
  const c = cfg(mode, companyId)
  const listHref = listHrefFromBack(c.path, param(sp, 'back'), CREDIT_QUERY_KEYS)
  const res = await loadItem<SalesCreditNote | SupplierCreditNote>(c.api, docId, page.notFound)
  if ('panel' in res) return res.panel
  const d = res.item
  const posted = d.status === 'posted'
  const entryHref = d.entry_id && can(ctx.permissions, 'ledger.entry:read') ? `${companyPath(companyId, 'accounting', 'entries')}/${d.entry_id}` : null
  const actions: DocAction[] = []
  if (posted && can(ctx.permissions, c.post)) actions.push({ kind: 'patch', status: 'cancelled', label: t.actions.cancel, confirm: format(t.actions.confirmCancel, { no: d.document_no }), danger: true })
  if (mode === 'sales' && posted) actions.push({ kind: 'link', href: `${c.path}/${d.id}/print`, label: t.actions.print, icon: 'print', primary: true })
  const sales = 'bill_to' in d ? d : null
  const pur = 'bill_from' in d ? d : null
  const party = sales ? sales.bill_to : pur!.bill_from
  const partyRef = sales ? sales.customer : pur!.supplier
  const invoiceNo = pur ? `${d.invoice.document_no} · ${pur.invoice.supplier_invoice_no}` : d.invoice.document_no
  const facts: [string, React.ReactNode][] = [
    [mode === 'sales' ? texts.sales.fields.customer : texts.purchasing.facts.billFrom, [`${partyRef.code} ${party.name || partyRef.name}`, party.tax_id, branchLabel(party.branch_no)].filter(Boolean).join(' · ')],
    [f.invoice, <EntryLink key="i" href={`${c.invoicesPath}/${d.invoice.id}`} no={invoiceNo} />],
    ...(sales ? ([[f.invoiceDate, dateText(sales.invoice_date)]] as [string, string][]) : []),
    ...(pur
      ? ([
          [f.supplierNo, pur.supplier_credit_note_no],
          [f.docDate, dateText(pur.credit_note_date)],
        ] as [string, string][])
      : []),
    [mode === 'sales' ? t.form.date : t.form.postingDate, dateText(d.date)],
    [f.kind, <KindBadge key="k" kind={d.kind} />],
    [f.reason, d.reason],
    [f.entry, <EntryLink key="e" href={entryHref} no={d.entry_no} />],
    [f.stockDocument, d.stock_document_no],
    [texts.sales.fields.description, d.description],
  ]
  return (
    <FormPage crumb={{ app: c.app, label: page.heading, href: listHref }} title={d.document_no} description={party.name || partyRef.name} aside={<CreditStatus status={d.status} />}>
      <div className="flex flex-col gap-6">
        <DocActions apiPath={`${c.api}/${d.id}`} listHref={listHref} docNo={d.document_no} actions={actions} />
        <Facts items={facts} />
        {sales && (
          <dl className="grid gap-3 rounded-(--radius-panel) border border-line bg-surface p-5 text-sm shadow-card sm:grid-cols-3">
            {(
              [
                [f.original, sales.original_totals.net],
                [f.corrected, sales.correct_totals.net],
                [f.difference, sales.totals.net],
              ] as const
            ).map(([label, v]) => (
              <div key={label}>
                <dt className="text-[13px] text-ink-muted">{label}</dt>
                <dd className="mt-0.5 text-[17px] font-semibold tabular-nums">{formatAmount(v)}</dd>
              </div>
            ))}
          </dl>
        )}
        <LinesTable
          rows={d.lines.map((l) => ({
            key: l.line_no,
            item: l.item,
            quantity: toUnits(l.quantity) === 0n ? '' : l.quantity,
            unitPrice: l.unit_price,
            amount: d.price_includes_vat ? l.amounts.gross : l.amounts.net,
          }))}
          totals={d.totals}
          vatRate={d.vat_rate}
          inclusive={d.price_includes_vat}
        />
      </div>
    </FormPage>
  )
}
