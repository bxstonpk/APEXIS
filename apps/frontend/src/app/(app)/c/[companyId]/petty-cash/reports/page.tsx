import type { Metadata } from 'next'
import Form from 'next/form'
import Link from 'next/link'
import type { ReactNode } from 'react'
import { Alert, Button, Pagination, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import type { CsvCell } from '@/features/accounting/csv'
import { formatAmount, plainAmount, sum } from '@/features/accounting/money'
import { ReportActions } from '@/features/accounting/ReportActions'
import { RefFilter } from '@/features/organization/pages'
import { pcApi, pcPage, type DocRef, type Ref, type Totals, type UserRef, type VoucherRow } from '@/features/petty-cash/types'
import { VoucherStatus } from '@/features/petty-cash/Views'
import { ReportTable } from '@/features/reports/ReportTable'
import { formatDate, today } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { backendGet, type ListMeta } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { loadRef } from '@/lib/server/list'
import { param, withQuery, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.pettyCash.reports.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

const BOOKS = ['petty-cash-voucher-register', 'petty-cash-fund-balance', 'petty-cash-input-vat'] as const
type Book = (typeof BOOKS)[number]

type RegisterData = { rows: VoucherRow[]; totals: Totals }
type BalanceRow = { fund: Ref; custodian: UserRef; imprest_amount: string; funded: boolean; cash_on_hand: string; pending_replenishment: string; requested: string }
type VatRow = { date: string; tax_invoice_no: string; voucher: DocRef; payee_name: string; tax_id: string; branch_no: string; base: string; vat: string }
type VatData = { rows: VatRow[]; totals: { base: string; vat: string } }

const num = 'px-3 py-2 text-right tabular-nums whitespace-nowrap print:py-0.5'
const totalCell = `${num} border-b-4 border-double border-ink/70`

/**
 * รายงานเงินสดย่อย — ?book=&start_date=&end_date=&fund_id=&page= ส่งต่อ GET /reports
 * ทะเบียนใบสำคัญจ่าย / ภาษีซื้อ: ช่วงวันที่; ยอดกองทุน: ณ วันที่
 */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.pettyCash.reports
  const c = t.columns
  const vc = texts.pettyCash.vouchers.columns
  const fc = texts.pettyCash.funds.fields
  const path = pcPage(companyId, 'reports')
  const vouchersPath = pcPage(companyId, 'vouchers')
  const book: Book = (BOOKS as readonly string[]).includes(param(sp, 'book')) ? (param(sp, 'book') as Book) : BOOKS[0]
  const end = param(sp, 'end_date') || today()
  const start = param(sp, 'start_date') || `${end.slice(0, 8)}01`
  const ranged = book !== 'petty-cash-fund-balance'
  const withFund = book === 'petty-cash-voucher-register'
  const query = new URLSearchParams({ book, page_size: '100', end_date: end })
  if (ranged) query.set('start_date', start)
  if (withFund && param(sp, 'fund_id')) query.set('fund_id', param(sp, 'fund_id'))
  if (param(sp, 'page')) query.set('page', param(sp, 'page'))
  const [res, fund] = await Promise.all([
    backendGet<unknown>(pcApi.reports(companyId), query).then(
      (r) => r as { data: unknown; meta?: ListMeta },
      (err: unknown) => ({ error: errorMessage(err) }),
    ),
    withFund ? loadRef(pcApi.funds(companyId), param(sp, 'fund_id')) : Promise.resolve(null),
  ])
  const period = ranged ? format(texts.reports.period, { start: formatDate(start), end: formatDate(end) }) : `${t.fields.asOf} ${formatDate(end)}`
  const head = [[ctx.company.name], [t.books[book], period], []]
  const voucherLink = (v: { id: string; document_no: string }) => (
    <Link href={`${vouchersPath}/${v.id}`} className="font-mono text-[13px] whitespace-nowrap text-primary hover:underline print:text-ink">
      {v.document_no}
    </Link>
  )
  let table: ReactNode = null
  let csv: CsvCell[][] = []

  if ('data' in res && book === 'petty-cash-voucher-register') {
    const d = res.data as RegisterData
    const rows = d.rows ?? []
    csv = [
      ...head,
      [vc.date, vc.no, vc.fund, vc.payee, vc.description, vc.taxInvoice, texts.pettyCash.vouchers.totals.net, texts.pettyCash.vouchers.totals.vat, vc.gross, texts.common.status],
      ...rows.map((r) => [
        r.date,
        r.document_no,
        r.fund.code,
        r.payee_name,
        r.description,
        r.tax_invoice_no,
        plainAmount(r.totals.net),
        plainAmount(r.totals.vat),
        plainAmount(r.totals.gross),
        texts.pettyCash.voucherStatuses[r.status] ?? r.status,
      ]),
      ['', '', '', '', '', c.total, plainAmount(d.totals.net), plainAmount(d.totals.vat), plainAmount(d.totals.gross), ''],
    ]
    table = (
      <ReportTable
        minWidth="60rem"
        head={[vc.date, vc.no, vc.fund, vc.payee, vc.taxInvoice]}
        numHead={[texts.pettyCash.vouchers.totals.net, texts.pettyCash.vouchers.totals.vat, vc.gross, texts.common.status]}
        empty={rows.length === 0}
        foot={
          <tr className="border-t-2 border-ink/70 font-semibold">
            <td colSpan={5} className="px-3 py-2 text-right">
              {c.total}
            </td>
            <td className={totalCell}>{formatAmount(d.totals.net)}</td>
            <td className={totalCell}>{formatAmount(d.totals.vat)}</td>
            <td className={totalCell}>{formatAmount(d.totals.gross)}</td>
            <td />
          </tr>
        }
      >
        {rows.map((r) => (
          <tr key={r.id} className="border-b border-line break-inside-avoid">
            <td className="px-3 py-2 whitespace-nowrap">{formatDate(r.date)}</td>
            <td className="px-3 py-2">{voucherLink(r)}</td>
            <td className="px-3 py-2 whitespace-nowrap">{r.fund.code}</td>
            <td className="px-3 py-2">
              <span className="block">{r.payee_name}</span>
              {r.description && <span className="block text-[12px] text-ink-muted">{r.description}</span>}
            </td>
            <td className="px-3 py-2 whitespace-nowrap">{r.tax_invoice_no || '–'}</td>
            <td className={num}>{formatAmount(r.totals.net)}</td>
            <td className={num}>{formatAmount(r.totals.vat, { dashZero: true })}</td>
            <td className={`${num} font-medium`}>{formatAmount(r.totals.gross)}</td>
            <td className="px-3 py-2 text-right">
              <VoucherStatus status={r.status} />
            </td>
          </tr>
        ))}
      </ReportTable>
    )
  }

  if ('data' in res && book === 'petty-cash-fund-balance') {
    const rows = ((res.data as { rows?: BalanceRow[] }).rows ?? []) as BalanceRow[]
    const tot = (k: 'imprest_amount' | 'cash_on_hand' | 'pending_replenishment' | 'requested') => sum(rows.map((r) => r[k]))
    csv = [
      ...head,
      [fc.code, fc.name, fc.custodian, fc.imprest, fc.cashOnHand, fc.pending, fc.requested],
      ...rows.map((r) => [r.fund.code, r.fund.name, r.custodian.name, plainAmount(r.imprest_amount), plainAmount(r.cash_on_hand), plainAmount(r.pending_replenishment), plainAmount(r.requested)]),
    ]
    table = (
      <ReportTable
        minWidth="52rem"
        head={[fc.code, fc.name, fc.custodian]}
        numHead={[fc.imprest, fc.cashOnHand, fc.pending, fc.requested]}
        empty={rows.length === 0}
        foot={
          <tr className="border-t-2 border-ink/70 font-semibold">
            <td colSpan={3} className="px-3 py-2 text-right">
              {c.total}
            </td>
            <td className={totalCell}>{formatAmount(tot('imprest_amount'))}</td>
            <td className={totalCell}>{formatAmount(tot('cash_on_hand'))}</td>
            <td className={totalCell}>{formatAmount(tot('pending_replenishment'))}</td>
            <td className={totalCell}>{formatAmount(tot('requested'))}</td>
          </tr>
        }
      >
        {rows.map((r) => (
          <tr key={r.fund.id} className="border-b border-line break-inside-avoid">
            <td className="px-3 py-2 font-mono text-[13px] whitespace-nowrap">{r.fund.code}</td>
            <td className="px-3 py-2">{r.fund.name}</td>
            <td className="px-3 py-2">{r.custodian.name}</td>
            <td className={num}>{formatAmount(r.imprest_amount)}</td>
            <td className={`${num} font-medium`}>{formatAmount(r.cash_on_hand)}</td>
            <td className={num}>{formatAmount(r.pending_replenishment, { dashZero: true })}</td>
            <td className={num}>{formatAmount(r.requested, { dashZero: true })}</td>
          </tr>
        ))}
      </ReportTable>
    )
  }

  if ('data' in res && book === 'petty-cash-input-vat') {
    const d = res.data as VatData
    const rows = d.rows ?? []
    csv = [
      ...head,
      [c.date, c.taxInvoiceNo, c.voucher, c.payee, c.taxId, c.branch, c.base, c.vat],
      ...rows.map((r) => [r.date, r.tax_invoice_no, r.voucher.document_no, r.payee_name, r.tax_id, r.branch_no, plainAmount(r.base), plainAmount(r.vat)]),
      ['', '', '', '', '', c.total, plainAmount(d.totals.base), plainAmount(d.totals.vat)],
    ]
    table = (
      <ReportTable
        minWidth="60rem"
        head={[c.date, c.taxInvoiceNo, c.voucher, c.payee, c.taxId, c.branch]}
        numHead={[c.base, c.vat]}
        empty={rows.length === 0}
        foot={
          <tr className="border-t-2 border-ink/70 font-semibold">
            <td colSpan={6} className="px-3 py-2 text-right">
              {c.total}
            </td>
            <td className={totalCell}>{formatAmount(d.totals.base)}</td>
            <td className={totalCell}>{formatAmount(d.totals.vat)}</td>
          </tr>
        }
      >
        {rows.map((r) => (
          <tr key={r.voucher.id} className="border-b border-line break-inside-avoid">
            <td className="px-3 py-2 whitespace-nowrap">{formatDate(r.date)}</td>
            <td className="px-3 py-2 whitespace-nowrap">{r.tax_invoice_no}</td>
            <td className="px-3 py-2">{voucherLink(r.voucher)}</td>
            <td className="px-3 py-2">{r.payee_name}</td>
            <td className="px-3 py-2 font-mono text-[13px] whitespace-nowrap">{r.tax_id || '–'}</td>
            <td className="px-3 py-2 whitespace-nowrap">{r.branch_no || '–'}</td>
            <td className={num}>{formatAmount(r.base)}</td>
            <td className={`${num} font-medium`}>{formatAmount(r.vat)}</td>
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
        {withFund && <RefFilter label={t.fields.fund} name="fund_id" source={pcApi.funds(companyId)} value={fund} />}
        <Button type="submit">{t.fields.show}</Button>
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
