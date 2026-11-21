import { ChevronLeft } from 'lucide-react'
import type { Metadata } from 'next'
import Link from 'next/link'
import { format, texts } from '@/config'
import { FormPage, loadItem } from '@/features/organization/pages'
import { PeriodTable } from '@/features/organization/PeriodTable'
import { FISCAL_PERIOD_CLOSE, orgResources, type FiscalYear } from '@/features/organization/resources'
import { formatDate } from '@/lib/format'
import { can } from '@/lib/permissions'
import { getCompanyContext } from '@/lib/server/company'
import { listHrefFromBack } from '@/lib/server/list'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.fiscalYears.pageTitle }

type Props = { params: Promise<{ companyId: string; itemId: string }>; searchParams: Promise<SearchParams> }

/** ปีบัญชี + 12 งวด (คิวรีเดียว: GET /fiscal-years/{id} ส่งงวดมาด้วย) — ปิด / เปิดงวดได้เมื่อมีสิทธิ์ fiscal_period:close */
export default async function FiscalYearPage({ params, searchParams }: Props) {
  const { companyId, itemId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const r = orgResources.fiscalYears
  const listHref = listHrefFromBack(r.page(companyId), param(await searchParams, 'back'), r.queryKeys)
  const res = await loadItem<FiscalYear>(r.api(companyId), itemId, texts.errors.fiscal_year_not_found!)
  if ('panel' in res) return res.panel
  const fy = res.item

  return (
    <FormPage crumb={{ label: texts.fiscalYears.heading, href: listHref }} title={format(texts.fiscalYears.title, { year: fy.year })} description={`${formatDate(fy.start_date)} – ${formatDate(fy.end_date)}`}>
      <PeriodTable apiPath={`/companies/${companyId}/fiscal-periods`} periods={fy.periods ?? []} canClose={can(ctx.permissions, FISCAL_PERIOD_CLOSE)} />
      <Link href={listHref} className="mt-6 inline-flex items-center gap-1 text-sm text-primary hover:underline">
        <ChevronLeft aria-hidden className="size-4" />
        {texts.common.back}
      </Link>
    </FormPage>
  )
}
