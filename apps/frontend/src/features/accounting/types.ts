import { companyPath } from '@/config/apps'

/** ข้อมูลอ้างอิง (id + code + name) ตาม backend ledger responses */
export type Ref = { id: string; code: string; name: string }

export const ACCOUNT_TYPES = ['asset', 'liability', 'equity', 'revenue', 'expense', 'suspense'] as const
export type AccountType = (typeof ACCOUNT_TYPES)[number]

export type Account = {
  id: string
  code: string
  name: string
  name_en: string
  type: AccountType
  parent: Ref | null
  postable: boolean
  status: 'active' | 'inactive'
  description: string
}

export type Journal = { id: string; code: string; name: string; number_template: string; status: 'active' | 'inactive' }

export type EntryStatus = 'draft' | 'posted' | 'reversed'

export type EntrySummary = {
  id: string
  entry_no: string | null
  date: string
  journal: Ref
  description: string
  reference: string
  status: EntryStatus
  total_debit: string
  total_credit: string
  posted_at: string | null
}

export type EntryLine = {
  line_no: number
  account: Ref
  description: string
  debit: string
  credit: string
  branch_id: string | null
  department_id: string | null
  cost_center_id: string | null
  project_site_id: string | null
}

export type Entry = EntrySummary & {
  fiscal_period_id: string | null
  reversal_of_id: string | null
  reversed_by_id: string | null
  created_by: string
  posted_by: string | null
  lines: EntryLine[]
}

// ---------------------------------------------------------------- รายงาน

export const BOOKS = ['trial-balance', 'balance-sheet', 'income-statement', 'general-ledger'] as const
export type Book = (typeof BOOKS)[number]

export type AccountInfo = { id: string; code: string; name: string; type: AccountType }

export type TrialBalance = {
  book: 'trial-balance'
  start_date: string
  end_date: string
  rows: { account: AccountInfo; opening_balance: string; debit: string; credit: string; closing_balance: string }[]
  total_debit: string
  total_credit: string
  prior_years_earnings: string
}

export type StatementLine = { account_id: string; code: string; name: string; level: number; is_header: boolean; amount: string }

export type BalanceSheet = {
  book: 'balance-sheet'
  as_of: string
  assets: StatementLine[]
  total_assets: string
  liabilities: StatementLine[]
  total_liabilities: string
  equity: StatementLine[]
  retained_earnings_prior_years: string
  current_year_earnings: string
  total_equity: string
  total_liabilities_and_equity: string
  suspense: StatementLine[]
  total_suspense: string
}

export type IncomeStatement = {
  book: 'income-statement'
  start_date: string
  end_date: string
  revenue: StatementLine[]
  total_revenue: string
  expenses: StatementLine[]
  total_expenses: string
  net_income: string
  /** บัญชีพัก: เคลื่อนไหวในช่วง (เดบิตเป็นบวก) — แสดงแยก ไม่รวมใน net_income */
  suspense: StatementLine[]
  total_suspense: string
}

export type GeneralLedger = {
  book: 'general-ledger'
  account: AccountInfo
  start_date: string
  end_date: string
  opening_balance: string
  lines: { entry_id: string; entry_no: string; date: string; description: string; reference: string; debit: string; credit: string; balance: string }[]
}

export type Report = TrialBalance | BalanceSheet | IncomeStatement | GeneralLedger

// ---------------------------------------------------------------- เส้นทาง / สิทธิ์

export const ACCOUNT_QUERY_KEYS = ['q', 'type', 'status', 'sort', 'page', 'page_size'] as const
export const JOURNAL_QUERY_KEYS = ['q', 'status', 'sort', 'page', 'page_size'] as const
export const ENTRY_QUERY_KEYS = ['q', 'status', 'journal_id', 'account_id', 'start_date', 'end_date', 'sort', 'page', 'page_size'] as const
export const REPORT_QUERY_KEYS = ['book', 'start_date', 'end_date', 'account_id', 'page', 'page_size'] as const

export const perms = {
  accountManage: 'ledger.account:manage',
  journalManage: 'ledger.journal:manage',
  entryManage: 'ledger.entry:manage',
  entryPost: 'ledger.entry:post',
} as const

export const ledgerApi = {
  accounts: (c: string) => `/companies/${c}/accounts`,
  accountTree: (c: string) => `/companies/${c}/accounts/tree`,
  accountDefaults: (c: string) => `/companies/${c}/accounts/defaults`,
  journals: (c: string) => `/companies/${c}/journals`,
  entries: (c: string) => `/companies/${c}/journal-entries`,
  reports: (c: string) => `/companies/${c}/reports`,
  branches: (c: string) => `/companies/${c}/branches`,
  departments: (c: string) => `/companies/${c}/departments`,
  costCenters: (c: string) => `/companies/${c}/cost-centers`,
  projectSites: (c: string) => `/companies/${c}/project-sites`,
}

export const ledgerPage = {
  accounts: (c: string) => companyPath(c, 'accounting', 'accounts'),
  journals: (c: string) => companyPath(c, 'accounting', 'journals'),
  entries: (c: string) => companyPath(c, 'accounting', 'entries'),
  reports: (c: string) => companyPath(c, 'accounting', 'reports'),
}
