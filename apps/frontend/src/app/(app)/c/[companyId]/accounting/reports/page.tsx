import type { Metadata } from 'next'
import Form from 'next/form'
import { Alert, Button, Pagination, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { ReportActions } from '@/features/accounting/ReportActions'
import { reportCsvRows } from '@/features/accounting/reportCsv'
import { DBDTable } from '@/features/accounting/DBDTable'
import { BalanceSheetTable, GeneralLedgerTable, IncomeStatementTable, TrialBalanceTable } from '@/features/accounting/ReportTables'
import { BOOKS, ledgerApi, ledgerPage, type Book, type Report } from '@/features/accounting/types'
import { RefFilter } from '@/features/organization/pages'
import { formatDate, today } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { backendGet, type ListMeta } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { loadRef } from '@/lib/server/list'
import { param, withQuery, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.reports.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/**
 * รายงานบัญชีหน้าเดียว — ?book=&start_date=&end_date=&account_id=&page= ส่งต่อ GET /reports ตรงๆ
 * ไม่ระบุวันที่ = ต้นปีปฏิทินถึงวันนี้ (แสดงทันที); บัญชีแยกประเภทต้องเลือกบัญชีก่อน
 * พิมพ์: ซ่อนเมนู / ตัวกรอง (print:hidden) เหลือหัวรายงาน + ตาราง; CSV สร้างจากข้อมูลที่แสดง
 */
export default async function ReportsPage({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.reports
  const path = ledgerPage.reports(companyId)

  const book: Book = (BOOKS as readonly string[]).includes(param(sp, 'book')) ? (param(sp, 'book') as Book) : 'trial-balance'
  const end = param(sp, 'end_date') || today()
  const start = param(sp, 'start_date') || `${end.slice(0, 4)}-01-01`
  const accountId = param(sp, 'account_id')
  const page = param(sp, 'page')
  const needsAccount = book === 'general-ledger' && !accountId

  const query = new URLSearchParams({ book, end_date: end })
  // งบ ณ วันที่ (ไม่ใช้ start_date): งบแสดงฐานะการเงิน และงบแบบกรมพัฒนาธุรกิจการค้า (กำไรขาดทุน = ต้นปีบัญชีถึงวันที่)
  const asOfBook = book === 'balance-sheet' || book === 'dbd-balance-sheet' || book === 'dbd-income-statement'
  if (!asOfBook) query.set('start_date', start)
  if (book === 'general-ledger') {
    query.set('account_id', accountId)
    query.set('page_size', '100')
    if (page) query.set('page', page)
  }

  const [res, account] = await Promise.all([
    needsAccount
      ? null
      : backendGet<Report>(ledgerApi.reports(companyId), query).then(
          (r) => r as { data: Report; meta?: ListMeta },
          (err: unknown) => ({ error: errorMessage(err) }),
        ),
    loadRef(ledgerApi.accounts(companyId), accountId),
  ])
  const report = res && 'data' in res ? res.data : null
  const period = asOfBook ? format(t.asOf, { date: formatDate(end) }) : format(t.period, { start: formatDate(start), end: formatDate(end) })
  const current = new URLSearchParams(query)
  current.delete('page_size')

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-wrap items-end justify-between gap-3 border-b border-line pb-4 print:border-ink/60">
        <div className="min-w-0">
          <p className="text-[13px] text-ink-muted print:text-ink">{ctx.company.name}</p>
          <h1 className="text-[22px] leading-tight font-semibold tracking-tight">{t.books[book]}</h1>
          <p className="mt-1 text-sm text-ink-muted">
            {period}
            {book === 'general-ledger' && account && ` · ${account.code} ${account.name}`}
          </p>
        </div>
        {report && <ReportActions rows={reportCsvRows(report, ctx.company.name)} filename={`${book}_${asOfBook ? end : `${start}_${end}`}.csv`} />}
      </header>

      <Form key={query.toString()} action="" className="flex flex-wrap items-end gap-2 rounded-(--radius-panel) border border-line bg-surface p-4 shadow-card print:hidden">
        <SelectField name="book" label={t.fields.book} defaultValue={book} options={BOOKS.map((b) => ({ value: b, label: t.books[b]! }))} className="w-full sm:w-56" />
        {!asOfBook && <TextField name="start_date" type="date" label={t.fields.start} defaultValue={start} />}
        <TextField name="end_date" type="date" label={asOfBook ? t.fields.asOf : t.fields.end} defaultValue={end} />
        {book === 'general-ledger' && <RefFilter label={t.fields.account} name="account_id" source={ledgerApi.accounts(companyId)} value={account} />}
        <Button type="submit">{t.run}</Button>      </Form>

      {res && 'error' in res && <Alert tone="error">{res.error}</Alert>}
      {report?.book === 'trial-balance' && <TrialBalanceTable r={report} ledgerHref={(id) => withQuery(path, { book: 'general-ledger', start_date: start, end_date: end, account_id: id })} />}
      {report?.book === 'balance-sheet' && <BalanceSheetTable r={report} />}
      {(report?.book === 'dbd-balance-sheet' || report?.book === 'dbd-income-statement') && <DBDTable r={report} />}
      {report?.book === 'income-statement' && <IncomeStatementTable r={report} />}
      {report?.book === 'general-ledger' && (
        <>
          <GeneralLedgerTable r={report} entryHref={(id) => `${ledgerPage.entries(companyId)}/${id}`} />
          {res && 'meta' in res && res.meta && res.meta.total > res.meta.page_size && (
            <div className="print:hidden">
              <Pagination
                page={res.meta.page}
                pageSize={res.meta.page_size}
                total={res.meta.total}
                href={(p) => withQuery(path, current, { page: p })}
                labels={texts.common.pagination}
              />
            </div>
          )}
        </>
      )}
    </div>
  )
}
