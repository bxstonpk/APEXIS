import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { format, texts } from '@/config'
import { AccountTree } from './AccountTree'
import { toCsv } from './csv'
import { EntryForm } from './EntryForm'
import { formatAmount, normalizeAmount, sum, toString, toUnits } from './money'
import { reportCsvRows } from './reportCsv'
import { branchIds, buildTree, initialExpanded, visibleRows } from './tree'
import type { Account, BalanceSheet, Entry } from './types'

const push = vi.fn()
vi.mock('next/navigation', () => ({ useRouter: () => ({ push, replace: vi.fn(), refresh: vi.fn() }) }))

afterEach(() => {
  push.mockReset()
  vi.unstubAllGlobals()
})

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
}

describe('money', () => {
  it('รวมทศนิยมโดยไม่มีเศษ float', () => {
    expect(toString(sum(['0.1', '0.2']))).toBe('0.3')
    expect(toString(toUnits('1,234.50'))).toBe('1234.5')
    expect(toUnits('abc')).toBe(0n)
  })

  it('ตรวจและปรับรูปแบบตัวเลขที่พิมพ์', () => {
    expect(normalizeAmount('1,000.50')).toBe('1000.5')
    expect(normalizeAmount('')).toBe('')
    expect(normalizeAmount('12a')).toBeNull()
    expect(normalizeAmount('1.1234567')).toBeNull() // เกิน 6 ตำแหน่ง
  })

  it('แสดงแบบงบการเงิน: คั่นหลัก, 2 ตำแหน่ง, ติดลบในวงเล็บ, ศูนย์เป็นขีด', () => {
    expect(formatAmount('1234567.5')).toBe('1,234,567.50')
    expect(formatAmount('-1000')).toBe('(1,000.00)')
    expect(formatAmount('0', { dashZero: true })).toBe('–')
    expect(formatAmount('0.125')).toBe('0.125')
  })
})

const acc = (code: string, parent: Account | null, postable = true): Account => ({
  id: `id-${code}`,
  code,
  name: `บัญชี ${code}`,
  name_en: '',
  type: 'asset',
  parent: parent && { id: parent.id, code: parent.code, name: parent.name },
  postable,
  status: 'active',
  description: '',
})

const root = acc('1000', null, false)
const group = acc('1100', root, false)
const cash = acc('1101', group)
const bank = acc('1102', group)
const accounts = [root, group, bank, cash] // ไม่เรียง — tree เรียงตามรหัสเอง

describe('tree', () => {
  it('สร้างลำดับชั้นจาก parent และนับระดับ', () => {
    const roots = buildTree(accounts)
    expect(roots).toHaveLength(1)
    expect(roots[0]!.children[0]!.children.map((n) => [n.account.code, n.depth])).toEqual([
      ['1101', 2],
      ['1102', 2],
    ])
    expect(branchIds(roots)).toEqual(['id-1000', 'id-1100'])
  })

  it('ค่าเริ่มต้นขยายระดับบนสุด; ยุบแล้วลูกไม่แสดง', () => {
    const roots = buildTree(accounts)
    expect(visibleRows(roots, initialExpanded(roots)).map((n) => n.account.code)).toEqual(['1000', '1100'])
    expect(visibleRows(roots, new Set(branchIds(roots))).map((n) => n.account.code)).toEqual(['1000', '1100', '1101', '1102'])
  })

  it('parent ที่หาไม่พบ = ระดับบนสุด (ไม่หายจากผัง)', () => {
    const orphan = { ...cash, parent: { id: 'gone', code: 'X', name: 'x' } }
    expect(buildTree([orphan]).map((n) => n.account.code)).toEqual(['1101'])
  })

  it('ปุ่มขยาย / ยุบใช้คีย์บอร์ดได้และบอกสถานะ', async () => {
    render(<AccountTree accounts={accounts} basePath="/c/x/accounting/accounts" />)
    expect(screen.queryByText('บัญชี 1101')).toBeNull()
    const toggle = screen.getByRole('button', { name: format(texts.accounts.expand, { name: 'บัญชี 1100' }) })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await userEvent.click(toggle)
    expect(screen.getByText('บัญชี 1101').closest('a')).toHaveAttribute('href', '/c/x/accounting/accounts/id-1101')
    await userEvent.click(screen.getByRole('button', { name: texts.accounts.collapseAll }))
    expect(screen.queryByText('บัญชี 1100')).toBeNull()
  })
})

describe('income statement csv', () => {
  it('บัญชีพักแสดงเป็นส่วนแยกหลังกำไรสุทธิ', () => {
    const line = (code: string, amount: string) => ({ account_id: code, code, name: code, level: 0, is_header: false, amount })
    const rows = reportCsvRows(
      { book: 'income-statement', start_date: '2026-01-01', end_date: '2026-06-30', revenue: [line('4100', '100')], total_revenue: '100',
        expenses: [], total_expenses: '0', net_income: '100', suspense: [line('6200', '-40')], total_suspense: '-40' },
      'ACME',
    )
    const net = rows.findIndex((r) => r[1] === texts.reports.totals.netIncome)
    const sus = rows.findIndex((r) => r[0] === texts.reports.sections.suspenseExcluded)
    expect(net).toBeGreaterThan(0)
    expect(sus).toBeGreaterThan(net)
    expect(rows).toContainEqual(['', texts.reports.totals.suspensePeriod, '-40.00'])
  })
})

describe('csv', () => {
  it('ครอบทุกช่อง, escape ", มี BOM และกันสูตร Excel', () => {
    expect(toCsv([['a"b', -5, '=SUM(A1)', null]])).toBe('﻿"a""b","-5","\'=SUM(A1)",""\r\n')
  })

  it('งบแสดงฐานะการเงินมีส่วนบัญชีพักเมื่อมียอด', () => {
    const line = (code: string, amount: string) => ({ account_id: code, code, name: code, level: 0, is_header: false, amount })
    const bs: BalanceSheet = {
      book: 'balance-sheet',
      as_of: '2026-12-31',
      assets: [line('1101', '1000')],
      total_assets: '1000',
      liabilities: [],
      total_liabilities: '0',
      equity: [],
      retained_earnings_prior_years: '0',
      current_year_earnings: '0',
      total_equity: '0',
      total_liabilities_and_equity: '0',
      suspense: [line('6400', '-1000')],
      total_suspense: '-1000',
    }
    const rows = reportCsvRows(bs, 'ACME')
    expect(rows[0]).toEqual(['ACME'])
    expect(rows).toContainEqual(['6400', '6400', '-1000.00'])
    expect(rows).toContainEqual(['', texts.reports.totals.suspense, '-1000.00'])
  })
})

const api = {
  entries: '/companies/c1/journal-entries',
  accounts: '/companies/c1/accounts',
  journals: '/companies/c1/journals',
  branches: '/companies/c1/branches',
  departments: '/companies/c1/departments',
  costCenters: '/companies/c1/cost-centers',
  projectSites: '/companies/c1/project-sites',
}
const tl = texts.entries.lines

const draft: Entry = {
  id: 'e1',
  entry_no: null,
  date: '2026-03-01',
  journal: { id: 'j1', code: 'GEN', name: 'สมุดรายวันทั่วไป' },
  description: 'ค่าเช่า',
  reference: '',
  status: 'draft',
  total_debit: '1200',
  total_credit: '1000',
  posted_at: null,
  fiscal_period_id: null,
  reversal_of_id: null,
  reversed_by_id: null,
  created_by: 'u1',
  posted_by: null,
  lines: [
    { line_no: 1, account: { id: 'a1', code: '5101', name: 'ค่าเช่า' }, description: '', debit: '1200', credit: '0', branch_id: 'b1', department_id: null, cost_center_id: null, project_site_id: null },
    { line_no: 2, account: { id: 'a2', code: '1101', name: 'เงินสด' }, description: '', debit: '0', credit: '1000', branch_id: null, department_id: null, cost_center_id: null, project_site_id: null },
  ],
}

describe('EntryForm', () => {
  const renderDraft = (canPost = true) =>
    render(
      <EntryForm
        companyApi={api}
        listHref="/c/c1/accounting/entries"
        entriesPath="/c/c1/accounting/entries"
        canPost={canPost}
        entry={draft}
        dimRefs={{ b1: { id: 'b1', code: 'HQ', name: 'สำนักงานใหญ่' } }}
      />,
    )

  it('แสดงผลต่างสดและสมดุลเมื่อแก้ยอด (กรอกด้านหนึ่งแล้วอีกด้านถูกล้าง)', async () => {
    renderDraft()
    expect(screen.getByText(format(tl.unbalanced, { amount: '200.00' }))).toBeInTheDocument()
    const credit2 = screen.getByLabelText(`${tl.credit} ${format(tl.line, { n: 2 })}`)
    await userEvent.clear(credit2)
    await userEvent.type(credit2, '1,200')
    expect(screen.getByText(tl.balanced)).toBeInTheDocument()
    const debit2 = screen.getByLabelText(`${tl.debit} ${format(tl.line, { n: 2 })}`)
    await userEvent.type(debit2, '5')
    expect(credit2).toHaveValue('')
  })

  it('มิติวิเคราะห์ซ่อนไว้ในบรรทัด กดแล้วจึงแสดง (บอกจำนวนที่ระบุไว้)', async () => {
    renderDraft()
    expect(screen.queryByLabelText(tl.branch)).toBeNull()
    const toggle = screen.getByRole('button', { name: `${tl.showDimensions} ${format(tl.line, { n: 1 })}` })
    expect(toggle).toHaveTextContent('1')
    await userEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByLabelText(tl.branch)).toHaveValue('HQ · สำนักงานใหญ่')
  })

  it('ผ่านรายการต้องสมดุล — ไม่ส่ง API เมื่อยังไม่สมดุล', async () => {
    const fetch = vi.fn()
    vi.stubGlobal('fetch', fetch)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    renderDraft()
    await userEvent.click(screen.getByRole('button', { name: texts.entries.actions.saveAndPost }))
    expect(fetch).not.toHaveBeenCalled()
    expect(screen.getByRole('alert')).toHaveTextContent(format(tl.unbalanced, { amount: '200.00' }))
  })

  it('บันทึกร่าง = PUT พร้อมมิติวิเคราะห์ แล้วไปหน้ารายการ', async () => {
    const fetch = vi.fn().mockResolvedValue(json(200, { data: { ...draft } }))
    vi.stubGlobal('fetch', fetch)
    renderDraft(false)
    await userEvent.click(screen.getByRole('button', { name: texts.entries.actions.saveDraft }))
    expect(fetch).toHaveBeenCalledTimes(1)
    const [url, init] = fetch.mock.calls[0] as [string, RequestInit]
    expect(url).toBe('/api/v1/companies/c1/journal-entries/e1')
    expect(init.method).toBe('PUT')
    const body = JSON.parse(init.body as string)
    expect(body.lines[0]).toMatchObject({ account_id: 'a1', debit: '1200', credit: '0', branch_id: 'b1' })
    expect(push).toHaveBeenCalledWith('/c/c1/accounting/entries/e1')
  })
})
