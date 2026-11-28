import type { Metadata } from 'next'
import Form from 'next/form'
import { Alert, Button, SelectField } from '@/components'
import { texts } from '@/config'
import { taxApi, type TaxOverview } from '@/features/finance/tax'
import { TaxOverviewTable } from '@/features/finance/TaxOverviewTable'
import { today } from '@/lib/format'
import { errorMessage } from '@/lib/http'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.finance.taxFilings.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ยื่นภาษีรายเดือน — ตาราง 12 เดือน x ภ.ง.ด.3 / ภ.ง.ด.53 / ภ.พ.30 ของปี (?year=) กดช่องเพื่อเปิดแบบของเดือนนั้น */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.finance.taxFilings
  const thisYear = Number(today().slice(0, 4))
  const raw = Number(param(sp, 'year'))
  const year = Number.isInteger(raw) && raw >= 2000 && raw <= 2200 ? raw : thisYear
  const res = await backendGet<TaxOverview>(taxApi.overview(companyId), new URLSearchParams({ year: String(year) })).then(
    (r) => ({ data: r.data }),
    (err: unknown) => ({ error: errorMessage(err) }),
  )
  const years = Array.from(new Set([thisYear + 1, thisYear, thisYear - 1, thisYear - 2, year])).sort((a, b) => b - a)

  return (
    <div className="flex flex-col gap-5">
      <header className="flex flex-wrap items-end justify-between gap-3 border-b border-line pb-4">
        <div>
          <p className="text-[13px] text-ink-muted">{ctx.company.name}</p>
          <h1 className="text-[22px] leading-tight font-semibold tracking-tight">{t.heading}</h1>
        </div>
        <Form action="" className="flex items-end gap-2">
          <SelectField name="year" label={t.year} defaultValue={String(year)} options={years.map((y) => ({ value: String(y), label: String(y + 543) }))} className="w-32" />
          <Button type="submit" variant="secondary">
            {t.show}
          </Button>
        </Form>
      </header>
      {'error' in res ? <Alert tone="error">{res.error}</Alert> : <TaxOverviewTable companyId={companyId} data={res.data} />}
    </div>
  )
}
