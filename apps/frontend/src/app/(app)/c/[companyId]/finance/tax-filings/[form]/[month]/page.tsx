import { Download, Printer } from 'lucide-react'
import type { Metadata } from 'next'
import { Alert, ButtonLink } from '@/components'
import { format, texts } from '@/config'
import { formatAmount } from '@/features/accounting/money'
import { FormPage } from '@/features/organization/pages'
import { MessagePanel } from '@/features/shell'
import { TaxFilingPanel } from '@/features/finance/TaxFilingPanel'
import { Pp30Lines } from '@/features/finance/TaxForms'
import {
  isMonth,
  isTaxForm,
  isWithholdingForm,
  monthLabel,
  taxApi,
  taxFormPage,
  taxPage,
  taxPerms,
  type Pp30,
  type TaxForm,
  type WhtForm,
} from '@/features/finance/tax'
import { formatDate } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { can } from '@/lib/permissions'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.finance.taxFilings.pageTitle }

type Props = { params: Promise<{ companyId: string; form: string; month: string }> }

/** แบบของเดือน — ภ.ง.ด.3 / 53 (สรุป + ใบแนบ + ไฟล์ยื่นแบบ) หรือ ภ.พ.30 (บรรทัดแบบ + รายงานภาษีขาย / ซื้อ) + การยื่น */
export default async function Page({ params }: Props) {
  const { companyId, form, month } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.finance.taxFilings
  if (!isTaxForm(form) || !isMonth(month)) return <MessagePanel title={texts.common.notFound} />
  const title = format(t.monthTitle, { form: t.forms[form]!, month: monthLabel(month) })
  const crumb = { app: texts.apps.finance.name, label: t.heading, href: `${taxPage(companyId)}?year=${month.slice(0, 4)}` }
  const canSee = can(ctx.permissions, taxPerms.read)
  const canManage = can(ctx.permissions, taxPerms.manage)
  const printHref = `${taxFormPage(companyId, form, month)}/print`
  const panel = (filing: WhtForm['filing'], amount: string, carry?: string) =>
    canSee ? (
      <TaxFilingPanel apiPath={taxApi.filings(companyId)} form={form as TaxForm} month={month} filing={filing} suggestedAmount={amount} suggestedCarry={carry} canManage={canManage} />
    ) : null

  if (isWithholdingForm(form)) {
    const res = await backendGet<WhtForm>(taxApi.withholding(companyId), new URLSearchParams({ form, month })).then(
      (r) => ({ data: r.data }),
      (err: unknown) => ({ error: errorMessage(err) }),
    )
    return (
      <FormPage crumb={crumb} title={title} description={t.formNames[form]}>
        {'error' in res ? (
          <Alert tone="error">{res.error}</Alert>
        ) : (
          <div className="flex flex-col gap-5">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <dl className="grid grid-cols-2 gap-x-8 gap-y-2 text-sm sm:grid-cols-4">
                <Fact label={t.summary.payees} value={String(res.data.summary.payees)} />
                <Fact label={t.summary.lines} value={String(res.data.summary.lines)} />
                <Fact label={t.summary.base} value={formatAmount(res.data.summary.base)} />
                <Fact label={t.summary.wht} value={formatAmount(res.data.summary.wht)} strong />
              </dl>
              <div className="flex flex-wrap gap-2">
                <ButtonLink href={printHref} variant="secondary">
                  <Printer aria-hidden />
                  {t.actions.print}
                </ButtonLink>
                {res.data.lines.length > 0 && (
                  <a
                    href={taxApi.exportUrl(companyId, form, month)}
                    download={`${form}_${month}.txt`}
                    className="inline-flex h-10 items-center gap-2 rounded-(--radius-control) border border-line-strong bg-surface px-4 text-sm font-medium hover:bg-subtle focus-visible:ring-2 focus-visible:ring-primary focus-visible:outline-none [&_svg]:size-4"
                  >
                    <Download aria-hidden />
                    {t.actions.export}
                  </a>
                )}
              </div>
            </div>
            <Attachment data={res.data} />
            {panel(res.data.filing, res.data.summary.wht)}
          </div>
        )}
      </FormPage>
    )
  }

  const res = await backendGet<Pp30>(taxApi.pp30(companyId), new URLSearchParams({ month })).then(
    (r) => ({ data: r.data }),
    (err: unknown) => ({ error: errorMessage(err) }),
  )
  return (
    <FormPage crumb={crumb} title={title} description={t.formNames[form]}>
      {'error' in res ? (
        <Alert tone="error">{res.error}</Alert>
      ) : (
        <div className="flex flex-col gap-5">
          <div className="flex flex-wrap justify-end gap-2">
            <ButtonLink href={printHref} variant="secondary">
              <Printer aria-hidden />
              {t.actions.print}
            </ButtonLink>
            <ButtonLink href={`${printHref}?report=output`} variant="secondary">
              <Printer aria-hidden />
              {t.actions.printOutput}
            </ButtonLink>
            <ButtonLink href={`${printHref}?report=input`} variant="secondary">
              <Printer aria-hidden />
              {t.actions.printInput}
            </ButtonLink>
          </div>
          <Pp30Lines data={res.data} />
          {panel(res.data.filing, res.data.payable, res.data.overpaid)}
        </div>
      )}
    </FormPage>
  )
}

function Fact({ label, value, strong = false }: { label: string; value: string; strong?: boolean }) {
  return (
    <div>
      <dt className="text-[12px] text-ink-muted">{label}</dt>
      <dd className={`tabular-nums ${strong ? 'text-base font-semibold' : 'font-medium'}`}>{value}</dd>
    </div>
  )
}

function Attachment({ data }: { data: WhtForm }) {
  const t = texts.finance.taxFilings
  const l = t.lines
  const th = 'px-3 py-2.5 text-left text-[13px] font-medium text-ink-muted whitespace-nowrap'
  const num = 'px-3 py-2 text-right tabular-nums whitespace-nowrap'
  return (
    <div className="relative overflow-x-auto rounded-(--radius-panel) border border-line bg-surface shadow-card">
      <table className="w-full min-w-[60rem] border-collapse text-sm">
        <caption className="sr-only">{format(t.attachment, { form: t.forms[data.form]! })}</caption>
        <thead>
          <tr className="border-b border-line bg-subtle">
            <th scope="col" className={`${th} text-right`}>{l.seq}</th>
            <th scope="col" className={th}>{l.taxId}</th>
            <th scope="col" className={th}>{l.name}</th>
            <th scope="col" className={th}>{l.date}</th>
            <th scope="col" className={th}>{l.income}</th>
            <th scope="col" className={`${th} text-right`}>{l.rate}</th>
            <th scope="col" className={`${th} text-right`}>{l.base}</th>
            <th scope="col" className={`${th} text-right`}>{l.wht}</th>
          </tr>
        </thead>
        <tbody>
          {data.lines.length === 0 && (
            <tr>
              <td colSpan={8} className="px-3 py-10 text-center text-ink-muted">{t.noRows}</td>
            </tr>
          )}
          {data.lines.map((r, i) => {
            const first = i === 0 || data.lines[i - 1]!.seq !== r.seq
            return (
              <tr key={`${r.payment_id}-${i}`} className={`border-line ${first ? 'border-t' : ''}`}>
                <td className={num}>{first ? r.seq : ''}</td>
                <td className="px-3 py-2 font-mono text-[13px] whitespace-nowrap">
                  {first && (
                    <>
                      {r.tax_id}
                      <span className="block font-sans text-[12px] text-ink-muted">{r.branch_no}</span>
                    </>
                  )}
                </td>
                <td className="px-3 py-2">
                  {first && (
                    <>
                      {r.name}
                      <span className="block text-[12px] text-ink-muted">{r.document_no}</span>
                    </>
                  )}
                </td>
                <td className="px-3 py-2 whitespace-nowrap">{first ? formatDate(r.date) : ''}</td>
                <td className="px-3 py-2 whitespace-nowrap">
                  {texts.finance.whtTypes.incomeTypesShort[r.income_type] ?? r.income_type}
                  {r.wht_type_name && <span className="block text-[12px] text-ink-muted">{r.wht_type_name}</span>}
                </td>
                <td className={num}>{r.rate}</td>
                <td className={num}>{formatAmount(r.base)}</td>
                <td className={num}>{formatAmount(r.wht)}</td>
              </tr>
            )
          })}
        </tbody>
        {data.lines.length > 0 && (
          <tfoot>
            <tr className="border-t-2 border-ink/70 font-semibold">
              <td colSpan={6} className="px-3 py-2 text-right">{t.grandTotal}</td>
              <td className={`${num} border-b-4 border-double border-ink/70`}>{formatAmount(data.summary.base)}</td>
              <td className={`${num} border-b-4 border-double border-ink/70`}>{formatAmount(data.summary.wht)}</td>
            </tr>
          </tfoot>
        )}
      </table>
    </div>
  )
}
