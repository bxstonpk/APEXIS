import type { Metadata } from 'next'
import Form from 'next/form'
import Link from 'next/link'
import type { ReactNode } from 'react'
import { Alert, Button, Pagination, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import type { CsvCell } from '@/features/accounting/csv'
import { formatAmount, plainAmount, toUnits } from '@/features/accounting/money'
import { ReportActions } from '@/features/accounting/ReportActions'
import { invApi, inventoryPage, type StockCard, type StockOnHand } from '@/features/inventory/types'
import { RefFilter } from '@/features/organization/pages'
import { ReportTable } from '@/features/reports/ReportTable'
import { formatDate, today } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { backendGet, type ListMeta } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { loadRef } from '@/lib/server/list'
import { param, withQuery, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.inventory.reports.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

const BOOKS = ['stock-on-hand', 'stock-card'] as const
type Book = (typeof BOOKS)[number]

const num = 'px-3 py-2 text-right tabular-nums whitespace-nowrap print:py-0.5'
const totalCell = `${num} border-b-4 border-double border-ink/70`

/**
 * รายงานคลัง — ?book=stock-on-hand&end_date=&warehouse_id=&category_id= / ?book=stock-card&item_id=&warehouse_id=&start_date=&end_date=
 * ส่งต่อ GET /reports ตรงๆ (กรอง / แบ่งหน้าใน DB); บัตรสินค้าต้องเลือกสินค้าก่อน
 */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.inventory.reports
  const c = t.columns
  const dt = texts.inventory.documents
  const path = inventoryPage(companyId, 'reports')
  const book: Book = (BOOKS as readonly string[]).includes(param(sp, 'book')) ? (param(sp, 'book') as Book) : BOOKS[0]
  const end = param(sp, 'end_date') || today()
  const start = param(sp, 'start_date') || `${end.slice(0, 8)}01`
  const itemId = param(sp, 'item_id')
  const warehouseId = param(sp, 'warehouse_id')
  const categoryId = param(sp, 'category_id')

  const query = new URLSearchParams({ book, end_date: end, page_size: '100' })
  if (warehouseId) query.set('warehouse_id', warehouseId)
  if (book === 'stock-on-hand' && categoryId) query.set('category_id', categoryId)
  if (book === 'stock-card') {
    query.set('start_date', start)
    if (itemId) query.set('item_id', itemId)
  }
  if (param(sp, 'page')) query.set('page', param(sp, 'page'))
  const needItem = book === 'stock-card' && !itemId

  const [res, item, warehouse, category] = await Promise.all([
    needItem
      ? Promise.resolve(null)
      : backendGet<unknown>(invApi.reports(companyId), query).then(
          (r) => r as { data: unknown; meta?: ListMeta },
          (err: unknown) => ({ error: errorMessage(err) }),
        ),
    loadRef(invApi.items(companyId), itemId),
    loadRef(invApi.warehouses(companyId), warehouseId),
    loadRef(invApi.categories(companyId), categoryId),
  ])
  const period = book === 'stock-on-hand' ? `${t.fields.asOf} ${formatDate(end)}` : format(texts.reports.period, { start: formatDate(start), end: formatDate(end) })
  const head = [[ctx.company.name], [t.books[book], period], []]
  let table: ReactNode = null
  let csv: CsvCell[][] = []
  let subtitle = ''

  if (res && 'data' in res && book === 'stock-on-hand') {
    const r = res.data as StockOnHand
    csv = [
      ...head,
      [c.item, c.uom, c.warehouse, c.quantity, c.value],
      ...r.rows.map((x) => [`${x.item.code} ${x.item.name}`, x.uom_code, x.warehouse.name, plainAmount(x.quantity), plainAmount(x.value)]),
      [t.total, '', '', '', plainAmount(r.total_value)],
    ]
    table = (
      <ReportTable
        minWidth="44rem"
        head={[c.item, c.uom, c.warehouse]}
        numHead={[c.quantity, c.value]}
        empty={r.rows.length === 0}
        foot={
          <tr className="border-t-2 border-ink/70 font-semibold">
            <td colSpan={4} className="px-3 py-2 text-right">
              {t.total}
            </td>
            <td className={totalCell}>{formatAmount(r.total_value)}</td>
          </tr>
        }
      >
        {r.rows.map((x) => (
          <tr key={`${x.item.id}-${x.warehouse.id}`} className="border-b border-line break-inside-avoid">
            <td className="px-3 py-2">
              <Link
                href={withQuery(path, new URLSearchParams(), { book: 'stock-card', item_id: x.item.id, warehouse_id: x.warehouse.id, end_date: end })}
                className="hover:text-primary hover:underline"
              >
                <span className="text-[13px] text-ink-muted">{x.item.code}</span> {x.item.name}
              </Link>
            </td>
            <td className="px-3 py-2 text-ink-muted">{x.uom_code}</td>
            <td className="px-3 py-2">{x.warehouse.name}</td>
            <td className={num}>{formatAmount(x.quantity)}</td>
            <td className={num}>{formatAmount(x.value)}</td>
          </tr>
        ))}
      </ReportTable>
    )
  }

  if (res && 'data' in res && book === 'stock-card') {
    const r = res.data as StockCard
    subtitle = `${r.item.code} ${r.item.name} (${r.uom_code}) · ${r.warehouse ? r.warehouse.name : t.fields.warehouse}`
    const docs = inventoryPage(companyId, 'documents')
    csv = [
      ...head,
      [subtitle],
      [c.date, c.no, c.type, c.warehouse, c.in, c.out, c.value, c.balance, c.balanceValue],
      [c.opening, '', '', '', '', '', '', plainAmount(r.opening_quantity), plainAmount(r.opening_value)],
      ...r.lines.map((l) => {
        const inbound = toUnits(l.quantity) > 0n
        return [
          l.date,
          l.document_no,
          dt.types[l.document_type] ?? l.document_type,
          l.warehouse.name,
          inbound ? plainAmount(l.quantity) : '',
          inbound ? '' : plainAmount(l.quantity.replace('-', '')),
          plainAmount(l.value),
          plainAmount(l.balance_quantity),
          plainAmount(l.balance_value),
        ]
      }),
    ]
    table = (
      <ReportTable minWidth="60rem" head={[c.date, c.no, c.type, c.warehouse]} numHead={[c.in, c.out, c.value, c.balance, c.balanceValue]} empty={false}>
        <tr className="border-b border-line bg-subtle/60">
          <td colSpan={7} className="px-3 py-2 font-medium">
            {c.opening}
          </td>
          <td className={num}>{formatAmount(r.opening_quantity)}</td>
          <td className={num}>{formatAmount(r.opening_value)}</td>
        </tr>
        {r.lines.length === 0 && (
          <tr>
            <td colSpan={9} className="px-3 py-8 text-center text-ink-muted">
              {t.noRows}
            </td>
          </tr>
        )}
        {r.lines.map((l, i) => {
          const inbound = toUnits(l.quantity) > 0n
          return (
            <tr key={`${l.document_id}-${i}`} className="border-b border-line break-inside-avoid">
              <td className="px-3 py-2 whitespace-nowrap">{formatDate(l.date)}</td>
              <td className="px-3 py-2 font-mono text-[13px] whitespace-nowrap">
                <Link href={`${docs}/${l.document_id}`} className="text-primary hover:underline print:text-ink">
                  {l.document_no}
                </Link>
              </td>
              <td className="px-3 py-2">{dt.types[l.document_type] ?? l.document_type}</td>
              <td className="px-3 py-2">{l.warehouse.name}</td>
              <td className={num}>{inbound ? formatAmount(l.quantity) : ''}</td>
              <td className={num}>{inbound ? '' : formatAmount(l.quantity.replace('-', ''))}</td>
              <td className={num}>{formatAmount(l.value)}</td>
              <td className={`${num} font-medium`}>{formatAmount(l.balance_quantity)}</td>
              <td className={num}>{formatAmount(l.balance_value)}</td>
            </tr>
          )
        })}
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
          <p className="mt-1 text-sm text-ink-muted">
            {period}
            {subtitle && ` · ${subtitle}`}
          </p>
        </div>
        {res && 'data' in res && <ReportActions rows={csv} filename={`${book}_${end}.csv`} />}
      </header>
      <Form key={query.toString()} action="" className="flex flex-wrap items-end gap-2 rounded-(--radius-panel) border border-line bg-surface p-4 shadow-card print:hidden">
        <SelectField name="book" label={t.fields.book} defaultValue={book} options={BOOKS.map((b) => ({ value: b, label: t.books[b]! }))} className="w-full sm:w-64" />
        {book === 'stock-card' && <RefFilter label={t.fields.item} name="item_id" source={invApi.items(companyId)} value={item} />}
        <RefFilter label={t.fields.warehouse} name="warehouse_id" source={invApi.warehouses(companyId)} value={warehouse} />
        {book === 'stock-on-hand' && <RefFilter label={t.fields.category} name="category_id" source={invApi.categories(companyId)} value={category} />}
        {book === 'stock-card' && <TextField name="start_date" type="date" label={t.fields.start} defaultValue={start} />}
        <TextField name="end_date" type="date" label={book === 'stock-on-hand' ? t.fields.asOf : t.fields.end} defaultValue={end} />
        <Button type="submit">{texts.reports.run}</Button>
      </Form>
      {needItem && <Alert>{t.pickItem}</Alert>}
      {res && 'error' in res && <Alert tone="error">{res.error}</Alert>}
      {table}
      {res && 'meta' in res && res.meta && res.meta.total > res.meta.page_size && (
        <div className="print:hidden">
          <Pagination page={res.meta.page} pageSize={res.meta.page_size} total={res.meta.total} href={(p) => withQuery(path, current, { page: p })} labels={texts.common.pagination} />
        </div>
      )}
    </div>
  )
}
