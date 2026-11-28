import type { Metadata } from 'next'
import Form from 'next/form'
import Link from 'next/link'
import type { ReactNode } from 'react'
import { Alert, Button, Pagination, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import type { CsvCell } from '@/features/accounting/csv'
import { formatAmount, plainAmount, sum, toString } from '@/features/accounting/money'
import { ReportActions } from '@/features/accounting/ReportActions'
import { mfgApi, mfgPage, type Ref } from '@/features/manufacturing/types'
import { OrderStatus } from '@/features/manufacturing/Views'
import { ReportTable } from '@/features/reports/ReportTable'
import { formatDate, today } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { backendGet, type ListMeta } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { param, withQuery, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.manufacturing.reports.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

const BOOKS = ['production-cost', 'material-usage', 'wip'] as const
type Book = (typeof BOOKS)[number]

type OrderRef = { id: string; document_no: string }
type CostRow = { order: OrderRef; date: string; item: Ref; quantity: string; materials: string; conversion: string; total: string; unit_cost: string }
type UsageRow = { order: OrderRef; item: Ref; standard_quantity: string; actual_quantity: string; variance_quantity: string; variance_value: string }
type WipRow = { order: OrderRef; item: Ref; status: string; materials: string; conversion: string; total: string }

const num = 'px-3 py-2 text-right tabular-nums whitespace-nowrap print:py-0.5'
const totalCell = `${num} border-b-4 border-double border-ink/70`

/** แถวรายงาน — รับได้ทั้ง { rows } และ array ตรงๆ */
function rowsOf<T>(data: unknown): T[] {
  if (Array.isArray(data)) return data as T[]
  return ((data as { rows?: T[] } | null)?.rows ?? []) as T[]
}

/**
 * รายงานการผลิต — ?book=&start_date=&end_date=&page= ส่งต่อ GET /reports
 * ต้นทุนการผลิต / การใช้วัตถุดิบ: ช่วงวันที่ผลิตเสร็จ; งานระหว่างผลิต: ณ วันที่
 */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.manufacturing.reports
  const c = t.columns
  const path = mfgPage(companyId, 'reports')
  const ordersPath = mfgPage(companyId, 'orders')
  const book: Book = (BOOKS as readonly string[]).includes(param(sp, 'book')) ? (param(sp, 'book') as Book) : BOOKS[0]
  const end = param(sp, 'end_date') || today()
  const start = param(sp, 'start_date') || `${end.slice(0, 8)}01`
  const ranged = book !== 'wip'
  const query = new URLSearchParams({ book, page_size: '100', end_date: end })
  if (ranged) query.set('start_date', start)
  if (param(sp, 'page')) query.set('page', param(sp, 'page'))
  const res = await backendGet<unknown>(mfgApi.reports(companyId), query).then(
    (r) => r as { data: unknown; meta?: ListMeta },
    (err: unknown) => ({ error: errorMessage(err) }),
  )
  const period = ranged ? format(texts.reports.period, { start: formatDate(start), end: formatDate(end) }) : `${t.fields.asOf} ${formatDate(end)}`
  const head = [[ctx.company.name], [t.books[book], period], []]
  const orderLink = (o: OrderRef) => (
    <Link href={`${ordersPath}/${o.id}`} className="font-mono text-[13px] whitespace-nowrap text-primary hover:underline print:text-ink">
      {o.document_no}
    </Link>
  )
  const itemCell = (i: Ref) => (
    <td className="px-3 py-2">
      <span className="text-[13px] text-ink-muted">{i.code}</span> {i.name}
    </td>
  )
  let table: ReactNode = null
  let csv: CsvCell[][] = []

  if ('data' in res && book === 'production-cost') {
    const rows = rowsOf<CostRow>(res.data)
    const tot = (k: 'materials' | 'conversion' | 'total') => sum(rows.map((r) => r[k]))
    csv = [
      ...head,
      [c.order, c.date, c.item, c.quantity, c.materials, c.conversion, c.total, c.unitCost],
      ...rows.map((r) => [r.order.document_no, r.date, `${r.item.code} ${r.item.name}`, plainAmount(r.quantity), plainAmount(r.materials), plainAmount(r.conversion), plainAmount(r.total), plainAmount(r.unit_cost)]),
      ['', '', t.total, '', plainAmount(toString(tot('materials'))), plainAmount(toString(tot('conversion'))), plainAmount(toString(tot('total'))), ''],
    ]
    table = (
      <ReportTable
        minWidth="56rem"
        head={[c.order, c.date, c.item]}
        numHead={[c.quantity, c.materials, c.conversion, c.total, c.unitCost]}
        empty={rows.length === 0}
        foot={
          <tr className="border-t-2 border-ink/70 font-semibold">
            <td colSpan={4} className="px-3 py-2 text-right">
              {t.total}
            </td>
            <td className={totalCell}>{formatAmount(tot('materials'))}</td>
            <td className={totalCell}>{formatAmount(tot('conversion'))}</td>
            <td className={totalCell}>{formatAmount(tot('total'))}</td>
            <td />
          </tr>
        }
      >
        {rows.map((r) => (
          <tr key={r.order.id} className="border-b border-line break-inside-avoid">
            <td className="px-3 py-2">{orderLink(r.order)}</td>
            <td className="px-3 py-2 whitespace-nowrap">{formatDate(r.date)}</td>
            {itemCell(r.item)}
            <td className={num}>{formatAmount(r.quantity)}</td>
            <td className={num}>{formatAmount(r.materials)}</td>
            <td className={num}>{formatAmount(r.conversion, { dashZero: true })}</td>
            <td className={`${num} font-medium`}>{formatAmount(r.total)}</td>
            <td className={num}>{formatAmount(r.unit_cost)}</td>
          </tr>
        ))}
      </ReportTable>
    )
  }

  if ('data' in res && book === 'material-usage') {
    const rows = rowsOf<UsageRow>(res.data)
    const totalVariance = sum(rows.map((r) => r.variance_value))
    csv = [
      ...head,
      [c.order, c.item, c.standard, c.actual, c.varianceQty, c.varianceValue],
      ...rows.map((r) => [r.order.document_no, `${r.item.code} ${r.item.name}`, plainAmount(r.standard_quantity), plainAmount(r.actual_quantity), plainAmount(r.variance_quantity), plainAmount(r.variance_value)]),
    ]
    table = (
      <ReportTable
        minWidth="52rem"
        head={[c.order, c.item]}
        numHead={[c.standard, c.actual, c.varianceQty, c.varianceValue]}
        empty={rows.length === 0}
        foot={
          <tr className="border-t-2 border-ink/70 font-semibold">
            <td colSpan={5} className="px-3 py-2 text-right">
              {t.total}
            </td>
            <td className={totalCell}>{formatAmount(totalVariance)}</td>
          </tr>
        }
      >
        {rows.map((r, i) => (
          <tr key={`${r.order.id}-${r.item.id}-${i}`} className="border-b border-line break-inside-avoid">
            <td className="px-3 py-2">{orderLink(r.order)}</td>
            {itemCell(r.item)}
            <td className={num}>{formatAmount(r.standard_quantity)}</td>
            <td className={num}>{formatAmount(r.actual_quantity)}</td>
            <td className={num}>{formatAmount(r.variance_quantity, { dashZero: true })}</td>
            <td className={num}>{formatAmount(r.variance_value, { dashZero: true })}</td>
          </tr>
        ))}
      </ReportTable>
    )
  }

  if ('data' in res && book === 'wip') {
    const rows = rowsOf<WipRow>(res.data)
    const tot = (k: 'materials' | 'conversion' | 'total') => sum(rows.map((r) => r[k]))
    csv = [
      ...head,
      [c.order, c.item, c.status, c.materials, c.conversion, c.total],
      ...rows.map((r) => [r.order.document_no, `${r.item.code} ${r.item.name}`, texts.manufacturing.statuses[r.status] ?? r.status, plainAmount(r.materials), plainAmount(r.conversion), plainAmount(r.total)]),
    ]
    table = (
      <ReportTable
        minWidth="48rem"
        head={[c.order, c.item, c.status]}
        numHead={[c.materials, c.conversion, c.total]}
        empty={rows.length === 0}
        foot={
          <tr className="border-t-2 border-ink/70 font-semibold">
            <td colSpan={3} className="px-3 py-2 text-right">
              {t.total}
            </td>
            <td className={totalCell}>{formatAmount(tot('materials'))}</td>
            <td className={totalCell}>{formatAmount(tot('conversion'))}</td>
            <td className={totalCell}>{formatAmount(tot('total'))}</td>
          </tr>
        }
      >
        {rows.map((r) => (
          <tr key={r.order.id} className="border-b border-line break-inside-avoid">
            <td className="px-3 py-2">{orderLink(r.order)}</td>
            {itemCell(r.item)}
            <td className="px-3 py-2">
              <OrderStatus status={r.status} />
            </td>
            <td className={num}>{formatAmount(r.materials)}</td>
            <td className={num}>{formatAmount(r.conversion, { dashZero: true })}</td>
            <td className={`${num} font-medium`}>{formatAmount(r.total)}</td>
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
          <p className="mt-1 text-sm text-ink-muted">{period}</p>
        </div>
        {'data' in res && <ReportActions rows={csv} filename={`${book}_${end}.csv`} />}
      </header>
      <Form key={query.toString()} action="" className="flex flex-wrap items-end gap-2 rounded-(--radius-panel) border border-line bg-surface p-4 shadow-card print:hidden">
        <SelectField name="book" label={t.fields.book} defaultValue={book} options={BOOKS.map((b) => ({ value: b, label: t.books[b]! }))} className="w-full sm:w-72" />
        {ranged && <TextField name="start_date" type="date" label={t.fields.start} defaultValue={start} />}
        <TextField name="end_date" type="date" label={ranged ? t.fields.end : t.fields.asOf} defaultValue={end} />
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
