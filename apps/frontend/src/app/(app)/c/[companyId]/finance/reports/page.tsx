import type { Metadata } from 'next'
import Form from 'next/form'
import { Alert, Button, Pagination, SelectField, TextField } from '@/components'
import { format, texts } from '@/config'
import { formatAmount, plainAmount } from '@/features/accounting/money'
import { ReportActions } from '@/features/accounting/ReportActions'
import { financeApi, financePage, type WithholdingReport } from '@/features/finance/types'
import { formatDate, today } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { backendGet, type ListMeta } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { param, withQuery, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.finance.reports.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

const BOOKS = ['withholding-tax', 'withholding-tax-received'] as const

/** รายงานภาษีหัก ณ ที่จ่าย (ภ.ง.ด.3 / 53) และภาษีถูกหัก — ?book=&form=&start_date=&end_date=&page= ส่งต่อ GET /reports ตรงๆ */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.finance.reports
  const c = t.columns
  const path = financePage(companyId, 'reports')
  const book = (BOOKS as readonly string[]).includes(param(sp, 'book')) ? param(sp, 'book') : BOOKS[0]
  const end = param(sp, 'end_date') || today()
  const start = param(sp, 'start_date') || `${end.slice(0, 8)}01`
  const form = book === 'withholding-tax' ? param(sp, 'form') : ''
  const query = new URLSearchParams({ book, start_date: start, end_date: end, page_size: '100' })
  if (form) query.set('form', form)
  if (param(sp, 'page')) query.set('page', param(sp, 'page'))
  const res = await backendGet<WithholdingReport>(financeApi.reports(companyId), query).then(
    (r) => r as { data: WithholdingReport; meta?: ListMeta },
    (err: unknown) => ({ error: errorMessage(err) }),
  )
  const report = 'data' in res ? res.data : null
  const paid = book === 'withholding-tax'
  const csv = report
    ? [
        [ctx.company.name],
        [t.books[book], `${start} – ${end}`, form ? t.forms[form] : ''],
        [],
        [c.date, c.no, c.party, c.taxId, c.branch, ...(paid ? [c.form, c.type, c.rate] : []), c.base, c.wht],
        ...report.rows.map((r) => [
          r.date, r.document_no, r.party.name, r.party.tax_id, r.party.branch_no,
          ...(paid ? [t.forms[r.tax_form] ?? r.tax_form, r.wht_type?.name ?? '', r.rate] : []),
          plainAmount(r.base), plainAmount(r.wht),
        ]),
        ['', '', t.total, '', '', ...(paid ? ['', '', ''] : []), plainAmount(report.total_base), plainAmount(report.total_wht)],
      ]
    : []
  const current = new URLSearchParams(query)
  current.delete('page_size')
  const th = 'px-3 py-2.5 text-left text-[13px] font-medium text-ink-muted print:py-1'
  const num = 'px-3 py-2 text-right tabular-nums whitespace-nowrap print:py-0.5'

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-wrap items-end justify-between gap-3 border-b border-line pb-4">
        <div>
          <p className="text-[13px] text-ink-muted print:text-ink">{ctx.company.name}</p>
          <h1 className="text-[22px] leading-tight font-semibold tracking-tight">{t.books[book]}</h1>
          <p className="mt-1 text-sm text-ink-muted">
            {format(texts.reports.period, { start: formatDate(start), end: formatDate(end) })}
            {form && ` · ${t.forms[form]}`}
          </p>
        </div>
        {report && <ReportActions rows={csv} filename={`${book}_${start}_${end}.csv`} />}
      </header>

      <Form action="" className="flex flex-wrap items-end gap-2 rounded-(--radius-panel) border border-line bg-surface p-4 shadow-card print:hidden">
        <SelectField name="book" label={t.fields.book} defaultValue={book} options={BOOKS.map((b) => ({ value: b, label: t.books[b]! }))} className="w-full sm:w-64" />
        {paid && <SelectField name="form" label={t.fields.form} defaultValue={form} options={Object.entries(t.forms).map(([value, label]) => ({ value, label }))} />}
        <TextField name="start_date" type="date" label={t.fields.start} defaultValue={start} />
        <TextField name="end_date" type="date" label={t.fields.end} defaultValue={end} />
        <Button type="submit">{texts.reports.run}</Button>
      </Form>

      {'error' in res && <Alert tone="error">{res.error}</Alert>}
      {report && (
        <div className="overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card print:border-0 print:shadow-none">
          <table className="w-full min-w-[52rem] border-collapse text-sm print:text-[11px]">
            <thead>
              <tr className="border-b border-line bg-subtle print:bg-transparent">
                <th scope="col" className={th}>{c.date}</th>
                <th scope="col" className={th}>{c.no}</th>
                <th scope="col" className={th}>{c.party}</th>
                <th scope="col" className={th}>{c.taxId}</th>
                {paid && <th scope="col" className={th}>{c.type}</th>}
                <th scope="col" className={`${th} text-right`}>{c.base}</th>
                <th scope="col" className={`${th} text-right`}>{c.wht}</th>
              </tr>
            </thead>
            <tbody>
              {report.rows.length === 0 && (
                <tr>
                  <td colSpan={paid ? 7 : 6} className="px-3 py-10 text-center text-ink-muted">{t.noRows}</td>
                </tr>
              )}
              {report.rows.map((r, i) => (
                <tr key={`${r.document_id}-${i}`} className="border-b border-line break-inside-avoid">
                  <td className="px-3 py-2 whitespace-nowrap">{formatDate(r.date)}</td>
                  <td className="px-3 py-2 font-mono text-[13px] whitespace-nowrap">{r.document_no}</td>
                  <td className="px-3 py-2">
                    {r.party.name}
                    {r.party.address && <span className="block text-[12px] text-ink-muted">{r.party.address}</span>}
                  </td>
                  <td className="px-3 py-2 font-mono text-[13px] whitespace-nowrap">
                    {r.party.tax_id || '–'}
                    {r.party.branch_no && <span className="block font-sans text-[12px] text-ink-muted">{r.party.branch_no}</span>}
                  </td>
                  {paid && (
                    <td className="px-3 py-2">
                      {r.wht_type?.name ?? '–'} <span className="text-ink-muted">{r.rate}%</span>
                      <span className="block text-[12px] text-ink-muted">{t.forms[r.tax_form] ?? r.tax_form}</span>
                    </td>
                  )}
                  <td className={num}>{formatAmount(r.base)}</td>
                  <td className={num}>{formatAmount(r.wht)}</td>
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr className="border-t-2 border-ink/70 font-semibold">
                <td colSpan={paid ? 5 : 4} className="px-3 py-2 text-right">{t.total}</td>
                <td className={`${num} border-b-4 border-double border-ink/70`}>{formatAmount(report.total_base)}</td>
                <td className={`${num} border-b-4 border-double border-ink/70`}>{formatAmount(report.total_wht)}</td>
              </tr>
            </tfoot>
          </table>
        </div>
      )}
      {'meta' in res && res.meta && res.meta.total > res.meta.page_size && (
        <div className="print:hidden">
          <Pagination page={res.meta.page} pageSize={res.meta.page_size} total={res.meta.total} href={(p) => withQuery(path, current, { page: p })} labels={texts.common.pagination} />
        </div>
      )}
    </div>
  )
}
