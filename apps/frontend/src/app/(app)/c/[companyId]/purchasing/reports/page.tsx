import type { Metadata } from 'next'
import Form from 'next/form'
import Link from 'next/link'
import type { ReactNode } from 'react'
import { Alert, Button, Pagination, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import type { CsvCell } from '@/features/accounting/csv'
import { formatAmount, plainAmount } from '@/features/accounting/money'
import { ReportActions } from '@/features/accounting/ReportActions'
import { purchasingDocsApi, purchasingPage, type ApAging, type InputVatReport, type PurchasingOutstandingRow } from '@/features/purchasing/types'
import { ReportTable } from '@/features/reports/ReportTable'
import { formatDate, today } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { backendGet, type ListMeta } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { param, withQuery, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.purchasing.reports.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

const BOOKS = ['outstanding-purchase-orders', 'ap-aging', 'input-vat'] as const
type Book = (typeof BOOKS)[number]

const num = 'px-3 py-2 text-right tabular-nums whitespace-nowrap print:py-0.5'
const totalCell = `${num} border-b-4 border-double border-ink/70`

/**
 * รายงานจัดซื้อ — ?book=&start_date=&end_date=&page= ส่งต่อ GET /reports
 * ค้างรับ / ค้างแจ้งหนี้ไม่ใช้วันที่; อายุเจ้าหนี้ ณ วันที่; ภาษีซื้อตามวันที่บันทึกบัญชี (ใบลดหนี้ผู้ขายเป็นยอดติดลบ)
 */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.purchasing.reports
  const c = t.columns
  const ag = texts.sales.reports.columns
  const path = purchasingPage(companyId, 'reports')
  const book: Book = (BOOKS as readonly string[]).includes(param(sp, 'book')) ? (param(sp, 'book') as Book) : BOOKS[0]
  const end = param(sp, 'end_date') || today()
  const start = param(sp, 'start_date') || `${end.slice(0, 8)}01`
  const query = new URLSearchParams({ book, page_size: '100' })
  if (book === 'ap-aging' || book === 'input-vat') query.set('end_date', end)
  if (book === 'input-vat') query.set('start_date', start)
  if (param(sp, 'page')) query.set('page', param(sp, 'page'))
  const res = await backendGet<unknown>(purchasingDocsApi.reports(companyId), query).then(
    (r) => r as { data: unknown; meta?: ListMeta },
    (err: unknown) => ({ error: errorMessage(err) }),
  )
  const period =
    book === 'ap-aging' ? `${t.fields.asOf} ${formatDate(end)}` : book === 'input-vat' ? format(texts.reports.period, { start: formatDate(start), end: formatDate(end) }) : ''
  const head = [[ctx.company.name], [t.books[book], period], []]
  let table: ReactNode = null
  let csv: CsvCell[][] = []

  if ('data' in res && book === 'outstanding-purchase-orders') {
    const rows = (res.data as { rows: PurchasingOutstandingRow[] }).rows
    csv = [
      ...head,
      [c.order, c.date, c.supplier, c.item, c.ordered, c.toReceive, c.toInvoice],
      ...rows.map((r) => [r.order.order_no, r.date, r.supplier.name, `${r.item.code} ${r.item.name}`, plainAmount(r.quantity), plainAmount(r.to_receive), plainAmount(r.to_invoice)]),
    ]
    table = (
      <ReportTable minWidth="48rem" head={[c.order, c.date, c.supplier, c.item]} numHead={[c.ordered, c.toReceive, c.toInvoice]} empty={rows.length === 0}>
        {rows.map((r) => (
          <tr key={`${r.order.id}-${r.line_no}`} className="border-b border-line break-inside-avoid">
            <td className="px-3 py-2 font-mono text-[13px] whitespace-nowrap">
              <Link href={`${purchasingPage(companyId, 'orders')}/${r.order.id}`} className="text-primary hover:underline print:text-ink">
                {r.order.order_no}
              </Link>
            </td>
            <td className="px-3 py-2 whitespace-nowrap">{formatDate(r.date)}</td>
            <td className="px-3 py-2">{r.supplier.name}</td>
            <td className="px-3 py-2">
              <span className="text-[13px] text-ink-muted">{r.item.code}</span> {r.item.name}
            </td>
            <td className={num}>{formatAmount(r.quantity)}</td>
            <td className={num}>{formatAmount(r.to_receive, { dashZero: true })}</td>
            <td className={num}>{formatAmount(r.to_invoice, { dashZero: true })}</td>
          </tr>
        ))}
      </ReportTable>
    )
  }

  if ('data' in res && book === 'ap-aging') {
    const a = res.data as ApAging
    const keys = ['not_due', 'days_1_30', 'days_31_60', 'days_61_90', 'over_90', 'total'] as const
    const labels = [ag.notDue, ag.d1, ag.d31, ag.d61, ag.d90, ag.total]
    csv = [...head, [c.supplier, ...labels], ...a.rows.map((r) => [`${r.supplier.code} ${r.supplier.name}`, ...keys.map((k) => plainAmount(r[k]))]), [t.total, ...keys.map((k) => plainAmount(a.totals[k]))]]
    table = (
      <ReportTable
        minWidth="52rem"
        head={[c.supplier]}
        numHead={labels}
        empty={a.rows.length === 0}
        foot={
          <tr className="border-t-2 border-ink/70 font-semibold">
            <td className="px-3 py-2 text-right">{t.total}</td>
            {keys.map((k) => (
              <td key={k} className={totalCell}>
                {formatAmount(a.totals[k])}
              </td>
            ))}
          </tr>
        }
      >
        {a.rows.map((r) => (
          <tr key={r.supplier.id} className="border-b border-line break-inside-avoid">
            <td className="px-3 py-2">
              <span className="text-[13px] text-ink-muted">{r.supplier.code}</span> {r.supplier.name}
            </td>
            {keys.map((k) => (
              <td key={k} className={`${num} ${k === 'total' ? 'font-medium' : ''}`}>
                {formatAmount(r[k], { dashZero: k !== 'total' })}
              </td>
            ))}
          </tr>
        ))}
      </ReportTable>
    )
  }

  if ('data' in res && book === 'input-vat') {
    const x = res.data as InputVatReport
    const docHref = (r: InputVatReport['rows'][number]) => `${purchasingPage(companyId, r.kind === 'credit_note' ? 'credit-notes' : 'invoices')}/${r.document_id}`
    csv = [
      ...head,
      [c.postingDate, c.no, c.taxInvoice, c.taxInvoiceDate, c.refNo, c.supplier, c.taxId, c.branch, c.base, c.tax],
      ...x.rows.map((r) => [r.date, r.document_no, r.tax_invoice_no, r.tax_invoice_date, r.reference_no, r.supplier.name, r.tax_id, r.branch_no, plainAmount(r.base), plainAmount(r.tax)]),
      ['', '', '', '', '', t.total, '', '', plainAmount(x.total_base), plainAmount(x.total_tax)],
    ]
    table = (
      <ReportTable
        minWidth="60rem"
        head={[c.postingDate, c.taxInvoice, c.supplier, c.taxId]}
        numHead={[c.base, c.tax]}
        empty={x.rows.length === 0}
        foot={
          <tr className="border-t-2 border-ink/70 font-semibold">
            <td colSpan={4} className="px-3 py-2 text-right">
              {t.total}
            </td>
            <td className={totalCell}>{formatAmount(x.total_base)}</td>
            <td className={totalCell}>{formatAmount(x.total_tax)}</td>
          </tr>
        }
      >
        {x.rows.map((r, i) => (
          <tr key={`${r.document_id}-${i}`} className="border-b border-line break-inside-avoid">
            <td className="px-3 py-2 whitespace-nowrap">
              {formatDate(r.date)}
              <Link href={docHref(r)} className="block font-mono text-[12px] text-primary hover:underline print:text-ink">
                {r.document_no}
              </Link>
            </td>
            <td className="px-3 py-2 whitespace-nowrap">
              <span className="font-mono text-[13px]">{r.tax_invoice_no}</span>
              <span className="block text-[12px] text-ink-muted">
                {formatDate(r.tax_invoice_date)}
                {r.kind === 'credit_note' && ` · ${texts.creditNotes.print.title} ${r.reference_no}`}
              </span>
            </td>
            <td className="px-3 py-2">{r.supplier.name}</td>
            <td className="px-3 py-2 font-mono text-[13px] whitespace-nowrap">
              {r.tax_id || '–'}
              {r.branch_no && <span className="block font-sans text-[12px] text-ink-muted">{r.branch_no}</span>}
            </td>
            <td className={num}>{formatAmount(r.base)}</td>
            <td className={num}>{formatAmount(r.tax)}</td>
          </tr>
        ))}
      </ReportTable>
    )
  }

  const current = new URLSearchParams(query)
  current.delete('page_size')
  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-wrap items-end justify-between gap-3 border-b border-line pb-4">
        <div>
          <p className="text-[13px] text-ink-muted print:text-ink">{ctx.company.name}</p>
          <h1 className="text-[22px] leading-tight font-semibold tracking-tight">{t.books[book]}</h1>
          {period && <p className="mt-1 text-sm text-ink-muted">{period}</p>}
        </div>
        {'data' in res && <ReportActions rows={csv} filename={`${book}_${end}.csv`} />}
      </header>
      <Form key={query.toString()} action="" className="flex flex-wrap items-end gap-2 rounded-(--radius-panel) border border-line bg-surface p-4 shadow-card print:hidden">
        <SelectField name="book" label={t.fields.book} defaultValue={book} options={BOOKS.map((b) => ({ value: b, label: t.books[b]! }))} className="w-full sm:w-72" />
        {book === 'input-vat' && <TextField name="start_date" type="date" label={t.fields.start} defaultValue={start} />}
        {book !== 'outstanding-purchase-orders' && <TextField name="end_date" type="date" label={book === 'ap-aging' ? t.fields.asOf : t.fields.end} defaultValue={end} />}
        <Button type="submit">{texts.reports.run}</Button>
      </Form>
      {'error' in res && <Alert tone="error">{res.error}</Alert>}
      {table}
      {'meta' in res && res.meta && res.meta.total > res.meta.page_size && (
        <div className="print:hidden">
          <Pagination page={res.meta.page} pageSize={res.meta.page_size} total={res.meta.total} href={(p) => withQuery(path, current, { page: p })} labels={texts.common.pagination} />
        </div>
      )}
    </div>
  )
}
