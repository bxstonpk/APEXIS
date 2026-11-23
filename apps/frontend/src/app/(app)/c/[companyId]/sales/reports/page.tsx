import type { Metadata } from 'next'
import Form from 'next/form'
import Link from 'next/link'
import type { ReactNode } from 'react'
import { Alert, Button, Pagination, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, plainAmount } from '@/features/accounting/money'
import { ReportActions } from '@/features/accounting/ReportActions'
import type { CsvCell } from '@/features/accounting/csv'
import { salesApi, salesPage, type Aging, type OutstandingRow, type TaxReport } from '@/features/sales/types'
import { formatDate, today } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { backendGet, type ListMeta } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { param, withQuery, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.reports.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

const BOOKS = ['outstanding-sales-orders', 'ar-aging', 'output-vat'] as const
type Book = (typeof BOOKS)[number]

const th = 'px-3 py-2.5 text-left text-[13px] font-medium text-ink-muted print:py-1'
const num = 'px-3 py-2 text-right tabular-nums whitespace-nowrap print:py-0.5'
const totalCell = `${num} border-b-4 border-double border-ink/70`

/**
 * รายงานขาย — ?book=&start_date=&end_date=&page= ส่งต่อ GET /reports ตรงๆ (กรอง / แบ่งหน้าใน DB, คิวรีเดียว)
 *   ค้างส่ง / ค้างออกบิล: ไม่ใช้วันที่; อายุลูกหนี้: ณ วันที่ (end_date); ภาษีขาย: ช่วงวันที่
 */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales.reports
  const c = t.columns
  const path = salesPage(companyId, 'reports')
  const book: Book = (BOOKS as readonly string[]).includes(param(sp, 'book')) ? (param(sp, 'book') as Book) : BOOKS[0]
  const end = param(sp, 'end_date') || today()
  const start = param(sp, 'start_date') || `${end.slice(0, 8)}01`
  const query = new URLSearchParams({ book, page_size: '100' })
  if (book !== 'outstanding-sales-orders') query.set('end_date', end)
  if (book === 'output-vat') query.set('start_date', start)
  if (param(sp, 'page')) query.set('page', param(sp, 'page'))
  const res = await backendGet<unknown>(salesApi.reports(companyId), query).then(
    (r) => r as { data: unknown; meta?: ListMeta },
    (err: unknown) => ({ error: errorMessage(err) }),
  )
  const period =
    book === 'output-vat' ? format(texts.reports.period, { start: formatDate(start), end: formatDate(end) }) : book === 'ar-aging' ? `${t.fields.asOf} ${formatDate(end)}` : ''

  let table: ReactNode = null
  let csv: CsvCell[][] = []
  const head = [[ctx.company.name], [t.books[book], period], []]
  const orderHref = (id: string) => `${salesPage(companyId, 'orders')}/${id}`

  if ('data' in res && book === 'outstanding-sales-orders') {
    const rows = (res.data as { rows: OutstandingRow[] }).rows
    csv = [
      ...head,
      [c.order, c.date, c.customer, c.item, c.ordered, c.toDeliver, c.toInvoice],
      ...rows.map((r) => [r.order.order_no, r.date, r.customer.name, `${r.item.code} ${r.item.name}`, plainAmount(r.quantity), plainAmount(r.to_deliver), plainAmount(r.to_invoice)]),
    ]
    table = (
      <Table minWidth="48rem" head={[c.order, c.date, c.customer, c.item]} numHead={[c.ordered, c.toDeliver, c.toInvoice]} empty={rows.length === 0}>
        {rows.map((r) => (
          <tr key={`${r.order.id}-${r.line_no}`} className="border-b border-line break-inside-avoid">
            <td className="px-3 py-2 font-mono text-[13px] whitespace-nowrap">
              <Link href={orderHref(r.order.id)} className="text-primary hover:underline print:text-ink">
                {r.order.order_no}
              </Link>
            </td>
            <td className="px-3 py-2 whitespace-nowrap">{formatDate(r.date)}</td>
            <td className="px-3 py-2">{r.customer.name}</td>
            <td className="px-3 py-2">
              <span className="text-[13px] text-ink-muted">{r.item.code}</span> {r.item.name}
            </td>
            <td className={num}>{formatAmount(r.quantity)}</td>
            <td className={num}>{formatAmount(r.to_deliver, { dashZero: true })}</td>
            <td className={num}>{formatAmount(r.to_invoice, { dashZero: true })}</td>
          </tr>
        ))}
      </Table>
    )
  }

  if ('data' in res && book === 'ar-aging') {
    const a = res.data as Aging
    const keys = ['not_due', 'days_1_30', 'days_31_60', 'days_61_90', 'over_90', 'total'] as const
    const labels = [c.notDue, c.d1, c.d31, c.d61, c.d90, c.total]
    csv = [...head, [c.customer, ...labels], ...a.rows.map((r) => [`${r.customer.code} ${r.customer.name}`, ...keys.map((k) => plainAmount(r[k]))]), [t.total, ...keys.map((k) => plainAmount(a.totals[k]))]]
    table = (
      <Table
        minWidth="52rem"
        head={[c.customer]}
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
          <tr key={r.customer.id} className="border-b border-line break-inside-avoid">
            <td className="px-3 py-2">
              <span className="text-[13px] text-ink-muted">{r.customer.code}</span> {r.customer.name}
            </td>
            {keys.map((k) => (
              <td key={k} className={`${num} ${k === 'total' ? 'font-medium' : ''}`}>
                {formatAmount(r[k], { dashZero: k !== 'total' })}
              </td>
            ))}
          </tr>
        ))}
      </Table>
    )
  }

  if ('data' in res && book === 'output-vat') {
    const x = res.data as TaxReport
    const invoiceHref = (id: string) => `${salesPage(companyId, 'invoices')}/${id}`
    csv = [
      ...head,
      [c.date, c.no, c.customer, c.taxId, c.branch, c.base, c.tax],
      ...x.rows.map((r) => [r.date, r.document_no, r.customer.name, r.tax_id, r.branch_no, plainAmount(r.base), plainAmount(r.tax)]),
      ['', '', t.total, '', '', plainAmount(x.total_base), plainAmount(x.total_tax)],
    ]
    table = (
      <Table
        minWidth="52rem"
        head={[c.date, c.no, c.customer, c.taxId]}
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
            <td className="px-3 py-2 whitespace-nowrap">{formatDate(r.date)}</td>
            <td className="px-3 py-2 font-mono text-[13px] whitespace-nowrap">
              <Link href={invoiceHref(r.document_id)} className="text-primary hover:underline print:text-ink">
                {r.document_no}
              </Link>
            </td>
            <td className="px-3 py-2">{r.customer.name}</td>
            <td className="px-3 py-2 font-mono text-[13px] whitespace-nowrap">
              {r.tax_id || '–'}
              {r.branch_no && <span className="block font-sans text-[12px] text-ink-muted">{r.branch_no}</span>}
            </td>
            <td className={num}>{formatAmount(r.base)}</td>
            <td className={num}>{formatAmount(r.tax)}</td>
          </tr>
        ))}
      </Table>
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

      <Form action="" className="flex flex-wrap items-end gap-2 rounded-(--radius-panel) border border-line bg-surface p-4 shadow-card print:hidden">
        <SelectField name="book" label={t.fields.book} defaultValue={book} options={BOOKS.map((b) => ({ value: b, label: t.books[b]! }))} className="w-full sm:w-72" />
        {book === 'output-vat' && <TextField name="start_date" type="date" label={t.fields.start} defaultValue={start} />}
        {book !== 'outstanding-sales-orders' && <TextField name="end_date" type="date" label={book === 'ar-aging' ? t.fields.asOf : t.fields.end} defaultValue={end} />}
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

function Table({ minWidth, head, numHead, empty, foot, children }: { minWidth: string; head: string[]; numHead: string[]; empty: boolean; foot?: ReactNode; children: ReactNode }) {
  return (
    <div className="overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card print:border-0 print:shadow-none">
      <table className="w-full border-collapse text-sm print:text-[11px]" style={{ minWidth }}>
        <thead>
          <tr className="border-b border-line bg-subtle print:bg-transparent">
            {head.map((h) => (
              <th key={h} scope="col" className={th}>
                {h}
              </th>
            ))}
            {numHead.map((h) => (
              <th key={h} scope="col" className={`${th} text-right`}>
                {h}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {empty ? (
            <tr>
              <td colSpan={head.length + numHead.length} className="px-3 py-10 text-center text-ink-muted">
                {texts.sales.reports.noRows}
              </td>
            </tr>
          ) : (
            children
          )}
        </tbody>
        {foot && !empty && <tfoot>{foot}</tfoot>}
      </table>
    </div>
  )
}
