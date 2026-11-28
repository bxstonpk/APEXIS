import { Alert } from '@/components'
import { format, texts } from '@/config'
import { companyPath } from '@/config/apps'
import { formatAmount, toString, toUnits } from '@/features/accounting/money'
import { FormPage, loadItem, NoPermission } from '@/features/organization/pages'
import { MessagePanel } from '@/features/shell'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'
import { DocActions, type DocAction } from './Actions'
import { loadLetterhead, loadVat, PaymentBadge, SaleDocList, SalesStatus, VatBadge } from './pages'
import { branchLabel, PrintDocument, printDate, type PrintTotal } from './PrintDocument'
import { salePrintTitle } from './printTitles'
import { PrintToolbar } from './PrintToolbar'
import { SaleForm } from './SaleForm'
import { TaxInvoiceAction } from './TaxInvoiceAction'
import { DOC_QUERY_KEYS, perms, SALE_MENU, saleApi, salesApi, salesPage, type Invoice, type Order, type SaleKind } from './types'
import { dateText, EntryLink, Facts, LinesTable } from './Views'

/**
 * หน้าของเอกสารขาย (server) — ขายสินค้า (invoices: ซื้อขาย / ผลิต) / ใบแจ้งหนี้ค่าบริการ (service-invoices: บริการ / ก่อสร้าง)
 * แอปขายออกเอกสารอย่างเดียว; การรับเงินยืนยันที่แอปการเงิน (ใบรับเงิน) — ใบแจ้งหนี้ค่าบริการรับเงินแล้วภาษีขายเกิดตามสัดส่วน
 */
export function salePageTexts(kind: SaleKind) {
  const t = texts.sales
  return kind === 'invoice' ? t.invoices : t.serviceInvoices
}

export async function SaleListPage({ companyId, sp, kind }: { companyId: string; sp: SearchParams; kind: SaleKind }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const page = salePageTexts(kind)
  const path = salesPage(companyId, SALE_MENU[kind])
  return (
    <SaleDocList
      companyId={companyId}
      sp={sp}
      kind={kind}
      path={path}
      apiPath={saleApi(companyId, kind)}
      heading={page.heading}      createHref={can(ctx.permissions, perms.invoicePost) && `${path}/new`}
    />
  )
}

/** สร้างเอกสารขาย — ?order_id= = จากใบสั่งขายที่อนุมัติแล้ว (คงเหลือ), ไม่มี = กรอกเอง */
export async function SaleNewPage({ companyId, kind, orderId }: { companyId: string; kind: SaleKind; orderId: string }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!can(ctx.permissions, perms.invoicePost)) return <NoPermission />
  const t = texts.sales
  const page = salePageTexts(kind)
  const path = salesPage(companyId, SALE_MENU[kind])
  let order: Order | undefined
  if (orderId) {
    const res = await loadItem<Order>(salesApi.orders(companyId), orderId, t.orders.notFound)
    if ('panel' in res) return res.panel
    order = res.item
  }
  const vat = order ? null : await loadVat(companyId)
  return (
    <FormPage crumb={{ app: texts.apps.sales.name, label: page.heading, href: path }} title={page.newTitle} description={order ? `${order.document_no ?? ''} · ${order.customer.name}` : undefined}>
      {order && order.status !== 'approved' ? (
        <MessagePanel title={t.orderNotApproved} />
      ) : (
        <SaleForm
          kind={kind}
          order={order}
          vat={vat}
          docsPath={path}
          listHref={order ? `${salesPage(companyId, 'orders')}/${order.id}` : path}
          apis={{
            submit: saleApi(companyId, kind),
            customers: salesApi.customers(companyId),
            items: salesApi.items(companyId),
            warehouses: salesApi.warehouses(companyId),
            openDeposits: `${salesApi.deposits(companyId)}/open`,
          }}
        />
      )}
    </FormPage>
  )
}

/** ดูเอกสารขาย — สถานะภาษีขาย, ยอดค้าง / รับเงิน (ยืนยันที่การเงิน), มัดจำที่หัก, ประวัติภาษีที่เกิด (ขายบริการ), ใบลดหนี้ */
export async function SaleViewPage({ companyId, kind, docId, sp }: { companyId: string; kind: SaleKind; docId: string; sp: SearchParams }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales
  const a = t.actions
  const page = salePageTexts(kind)
  const path = salesPage(companyId, SALE_MENU[kind])
  const api = saleApi(companyId, kind)
  const listHref = listHrefFromBack(path, param(sp, 'back'), DOC_QUERY_KEYS)
  const res = await loadItem<Invoice>(api, docId, page.notFound)
  if ('panel' in res) return res.panel
  const inv = res.item
  const posted = inv.status === 'posted'
  const has = (p: string) => can(ctx.permissions, p)
  const open = posted && toUnits(inv.outstanding) > 0n
  const entryHref = inv.entry_id && has('ledger.entry:read') ? `${companyPath(companyId, 'accounting', 'entries')}/${inv.entry_id}` : null
  const service = kind === 'service_invoice'

  const actions: DocAction[] = []
  if (posted && has(perms.invoicePost)) actions.push({ kind: 'patch', status: 'cancelled', label: a.cancelDoc, confirm: a.confirm.cancelDoc, danger: true })
  actions.push({ kind: 'link', href: `${path}/${inv.id}/print`, label: a.print, icon: 'print' })
  if (service && inv.tax_invoice)
    actions.push({ kind: 'link', href: `${path}/${inv.id}/print?as=tax-invoice`, label: t.serviceInvoices.taxInvoice.print, icon: 'print' })
  if (open && has(perms.invoicePost))
    actions.push({ kind: 'link', href: `${salesPage(companyId, 'credit-notes')}/new?invoice_id=${inv.id}&kind=${kind}`, label: texts.creditNotes.sales.create, icon: 'invoice' })

  const deposits = inv.deposits_applied ?? []
  return (
    <FormPage
      crumb={{ app: texts.apps.sales.name, label: page.heading, href: listHref }}
      title={inv.document_no}
      description={inv.customer.name}
      aside={
        <span className="flex flex-wrap items-center gap-2">
          {posted && <VatBadge status={inv.vat_status} />}
          <PaymentBadge outstanding={inv.outstanding} status={inv.status} />
          <SalesStatus kind="posted" status={inv.status} />
        </span>
      }
    >
      <div className="flex flex-col gap-6">
        <DocActions apiPath={`${api}/${inv.id}`} listHref={listHref} docNo={inv.document_no} actions={actions} />
        {service && posted && !inv.tax_invoice && toUnits(inv.vat_undue ?? '0') > 0n && has(perms.invoicePost) && (
          <TaxInvoiceAction apiPath={`${api}/${inv.id}`} docNo={inv.document_no} />
        )}
        <Facts
          items={[
            [t.fields.customer, `${inv.customer.code} ${inv.bill_to.name || inv.customer.name}`],
            [t.fields.customerBranch, branchLabel(inv.bill_to.branch_no)],
            [t.fields.date, dateText(inv.date)],
            [t.columns.due, dateText(inv.due_date)],
            [t.columns.order, inv.order ? <EntryLink key="o" href={`${salesPage(companyId, 'orders')}/${inv.order.id}`} no={inv.order.order_no} /> : ''],
            [t.fields.entry, <EntryLink key="e" href={entryHref} no={inv.entry_no} />],
            [t.fields.received, formatAmount(inv.received_amount)],
            [
              texts.creditNotes.columns.total,
              toUnits(inv.credited_amount) > 0n ? <EntryLink key="cn" href={`${salesPage(companyId, 'credit-notes')}?invoice_id=${inv.id}`} no={formatAmount(inv.credited_amount)} /> : '',
            ],
            [t.columns.outstanding, posted ? formatAmount(inv.outstanding) : ''],
            ...(service
              ? ([
                  [t.fields.vatRecognized, formatAmount(inv.vat_recognized)],
                  [t.fields.vatUndue, formatAmount(inv.vat_undue)],
                  [t.fields.taxInvoice, inv.tax_invoice ? `${inv.tax_invoice.no} · ${dateText(inv.tax_invoice.date)}` : ''],
                ] as [string, string][])
              : ([[t.fields.stockDocument, inv.stock_document_no]] as [string, string][])),
            [t.fields.reference, inv.reference],
          ]}
        />
        <LinesTable
          rows={inv.lines.map((l) => ({
            key: l.line_no,
            item: l.item,
            description: l.description,
            quantity: l.quantity,
            unitPrice: l.unit_price,
            amount: inv.price_includes_vat ? l.amounts.gross : l.amounts.net,
            extra:
              toUnits(l.credited_amount) > 0n ? (
                <span className="mt-1 block text-[12px] text-ink-muted">
                  {texts.creditNotes.form.credited} {formatAmount(l.credited_amount)}
                  {toUnits(l.returned_quantity) > 0n && ` · ${texts.creditNotes.form.returned} ${formatAmount(l.returned_quantity)}`}
                </span>
              ) : undefined,
          }))}
          totals={inv.totals}
          vatRate={inv.vat_rate}
          inclusive={inv.price_includes_vat}
        />
        {deposits.length > 0 && (
          <section aria-labelledby="sale-deposits-applied" className="flex flex-col gap-3">
            <h2 id="sale-deposits-applied" className="text-[15px] font-semibold">
              {t.fields.depositsApplied}
            </h2>
            <ul className="divide-y divide-line rounded-(--radius-panel) border border-line bg-surface text-sm shadow-card">
              {deposits.map((d) => (
                <li key={d.deposit.id} className="flex flex-wrap items-center justify-between gap-3 px-4 py-3">
                  <EntryLink href={`${salesPage(companyId, 'deposits')}/${d.deposit.id}`} no={d.deposit.document_no} />
                  <span className="tabular-nums">
                    {formatAmount(d.amount)} + {texts.sales.deposits.fields.vat} {formatAmount(d.vat)}
                  </span>
                </li>
              ))}
            </ul>
          </section>
        )}
        {service && posted && <Recognitions inv={inv} />}
      </div>
    </FormPage>
  )
}

function Recognitions({ inv }: { inv: Invoice }) {
  const t = texts.sales
  const r = t.serviceInvoices.recognitions
  const rows = inv.vat_recognitions ?? []
  return (
    <section aria-labelledby="sale-recognitions" className="flex flex-col gap-3">
      <h2 id="sale-recognitions" className="text-[15px] font-semibold">
        {t.sections.recognitions}
      </h2>
      <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
        {rows.length === 0 ? (
          <p className="px-4 py-6 text-center text-sm text-ink-muted">{r.empty}</p>
        ) : (
          <table className="w-full min-w-[36rem] border-collapse text-sm">
            <thead>
              <tr className="border-b border-line bg-subtle text-left text-[13px] text-ink-muted">
                <th scope="col" className="px-3 py-2.5 font-medium">
                  {t.columns.date}
                </th>
                <th scope="col" className="px-3 py-2.5 font-medium">
                  {t.columns.no}
                </th>
                <th scope="col" className="px-3 py-2.5 text-right font-medium">
                  {r.base}
                </th>
                <th scope="col" className="px-3 py-2.5 text-right font-medium">
                  {r.vat}
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.map((x, i) => (
                <tr key={`${x.document.id}-${i}`} className="border-b border-line last:border-b-0">
                  <td className="px-3 py-2.5 whitespace-nowrap">{dateText(x.date)}</td>
                  <td className="px-3 py-2.5">
                    <span className="font-mono text-[13px]">
                      {/* ใบรับเงินอยู่ที่แอปการเงิน — แอปขายไม่ลิงก์ไปการเงิน แสดงเลขที่อย่างเดียว */}
                      <EntryLink href={null} no={x.document.document_no} />
                    </span>
                    <span className="block text-[12px] text-ink-muted">{r.kinds[x.kind] ?? x.kind}</span>
                  </td>
                  <td className="px-3 py-2.5 text-right tabular-nums">{formatAmount(x.base)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums">{formatAmount(x.vat)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </section>
  )
}

/** พิมพ์เอกสารขาย A4 — ชื่อเอกสารตาม salePrintTitle; ?as=tax-invoice = ใบกำกับภาษีที่ออกก่อนรับเงินของใบแจ้งหนี้ค่าบริการ */
export async function SalePrintPage({ companyId, kind, docId, as }: { companyId: string; kind: SaleKind; docId: string; as: string }) {
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const p = texts.sales.print
  const [res, letterhead] = await Promise.all([loadItem<Invoice>(saleApi(companyId, kind), docId, salePageTexts(kind).notFound), loadLetterhead(companyId)])
  if ('panel' in res) return res.panel
  const inv = res.item
  if (!letterhead) return <Alert tone="error">{p.unavailable}</Alert>
  const vat = toUnits(inv.vat_rate) > 0n
  const asTax = kind === 'service_invoice' && as === 'tax-invoice' && !!inv.tax_invoice
  const deposits = inv.deposits_applied ?? []
  const depositTotal = deposits.reduce((s, d) => s + toUnits(d.amount) + toUnits(d.vat), 0n)
  // หักมัดจำ (รวม VAT ของมัดจำ) แล้วแสดงยอดที่ต้องชำระ — จำนวนเงินตัวอักษร = ยอดที่ต้องชำระ
  const payable = unitsToDecimal(toUnits(inv.totals.gross) - depositTotal)
  const totals: PrintTotal[] = [
    { label: p.subtotal, value: inv.totals.net },
    ...(vat ? [{ label: format(p.vat, { rate: inv.vat_rate }), value: inv.totals.vat }] : []),
    { label: p.grand, value: inv.totals.gross, strong: depositTotal === 0n },
    ...deposits.map((d) => ({ label: format(p.lessDeposit, { no: d.deposit.document_no }), value: unitsToDecimal(-(toUnits(d.amount) + toUnits(d.vat))) })),
    ...(depositTotal > 0n ? [{ label: texts.sales.totals.payable, value: payable, strong: true }] : []),
  ]
  return (
    <>
      <PrintToolbar backHref={`${salesPage(companyId, SALE_MENU[kind])}/${inv.id}`} />
      {inv.status === 'cancelled' && (
        <div className="mx-auto mb-4 max-w-[210mm] print:hidden">
          <Alert tone="error">{p.cancelled}</Alert>
        </div>
      )}
      <PrintDocument
        docType={kind === 'invoice' ? 'sales.invoice' : asTax ? 'sales.tax_invoice' : 'sales.service_invoice'}
        letterhead={letterhead}
        title={salePrintTitle(kind, vat, asTax)}
        docNo={asTax ? inv.tax_invoice!.no : inv.document_no}
        info={[
          [p.date, printDate(asTax ? inv.tax_invoice!.date : inv.date)],
          ...(asTax ? [] : ([[p.dueDate, printDate(inv.due_date)]] as [string, string][])),
          ...(asTax ? ([[p.refInvoice, inv.document_no]] as [string, string][]) : ([[p.orderNo, inv.order?.order_no ?? '']] as [string, string][])),
          [p.reference, inv.reference],
        ]}
        party={{ label: p.customer, name: inv.bill_to.name || inv.customer.name, taxId: inv.bill_to.tax_id, branchNo: inv.bill_to.branch_no, address: inv.bill_to.address }}
        lines={inv.lines.map((l) => ({
          description: l.item.name,
          detail: l.description || undefined,
          quantity: l.quantity,
          unitPrice: l.unit_price,
          amount: inv.price_includes_vat ? l.amounts.gross : l.amounts.net,
        }))}
        totals={totals}
        amountInWords={payable}
        note={[kind === 'service_invoice' && !asTax && vat ? p.noTaxInvoiceNote : '', inv.description].filter(Boolean).join(' — ') || undefined}
        signatures={[p.customerSign, p.approver]}
      />
    </>
  )
}

/** หน่วยย่อย (สเกล 10^6) → สตริงทศนิยม */
const unitsToDecimal = (units: bigint) => toString(units)
