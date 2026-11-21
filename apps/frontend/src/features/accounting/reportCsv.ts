import { texts } from '@/config'
import type { CsvCell } from './csv'
import { plainAmount } from './money'
import type { Report, StatementLine } from './types'

/**
 * แถว CSV ของรายงาน (pure) — ตัวเลขเป็นทศนิยมล้วน (ไม่คั่นหลัก, ลบใช้ -) ให้ Excel คำนวณต่อได้
 * แถวแรก = ชื่อรายงาน + ช่วงวันที่, ตามด้วยหัวคอลัมน์และข้อมูล
 */
export function reportCsvRows(r: Report, companyName: string): CsvCell[][] {
  const t = texts.reports
  const c = t.columns
  const title = (period: string): CsvCell[][] => [[companyName], [t.books[r.book] ?? r.book, period], []]
  const section = (name: string, lines: StatementLine[], totalLabel: string, total: string): CsvCell[][] => [
    [name],
    ...lines.map((l) => [l.code, `${'  '.repeat(l.level)}${l.name}`, plainAmount(l.amount)]),
    ['', totalLabel, plainAmount(total)],
    [],
  ]

  switch (r.book) {
    case 'trial-balance':
      return [
        ...title(`${r.start_date} – ${r.end_date}`),
        [c.code, c.account, c.opening, c.debit, c.credit, c.closing],
        ...r.rows.map((x) => [x.account.code, x.account.name, plainAmount(x.opening_balance), plainAmount(x.debit), plainAmount(x.credit), plainAmount(x.closing_balance)]),
        ['', t.total, '', plainAmount(r.total_debit), plainAmount(r.total_credit), ''],
      ]
    case 'balance-sheet':
      return [
        ...title(r.as_of),
        [c.code, c.account, c.amount],
        ...section(t.sections.assets, r.assets, t.totals.assets, r.total_assets),
        ...section(t.sections.liabilities, r.liabilities, t.totals.liabilities, r.total_liabilities),
        [t.sections.equity],
        ...r.equity.map((l) => [l.code, `${'  '.repeat(l.level)}${l.name}`, plainAmount(l.amount)]),
        ['', t.retainedPrior, plainAmount(r.retained_earnings_prior_years)],
        ['', t.currentYear, plainAmount(r.current_year_earnings)],
        ['', t.totals.equity, plainAmount(r.total_equity)],
        [],
        ['', t.totals.liabilitiesAndEquity, plainAmount(r.total_liabilities_and_equity)],
        ...(r.suspense.length ? [[], ...section(t.sections.suspense, r.suspense, t.totals.suspense, r.total_suspense)] : []),
      ]
    case 'income-statement':
      return [
        ...title(`${r.start_date} – ${r.end_date}`),
        [c.code, c.account, c.amount],
        ...section(t.sections.revenue, r.revenue, t.totals.revenue, r.total_revenue),
        ...section(t.sections.expenses, r.expenses, t.totals.expenses, r.total_expenses),
        ['', t.totals.netIncome, plainAmount(r.net_income)],
      ]
    case 'general-ledger':
      return [
        ...title(`${r.start_date} – ${r.end_date}`),
        [r.account.code, r.account.name],
        [c.date, c.no, c.description, c.debit, c.credit, c.balance],
        ['', '', t.opening, '', '', plainAmount(r.opening_balance)],
        ...r.lines.map((l) => [l.date, l.entry_no, l.description, plainAmount(l.debit), plainAmount(l.credit), plainAmount(l.balance)]),
      ]
  }
}
