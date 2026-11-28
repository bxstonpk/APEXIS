import type { Metadata } from 'next'
import { Alert } from '@/components'
import { texts } from '@/config'
import { isMonth, isTaxForm, isWithholdingForm, taxApi, taxFormPage, type Pp30, type VatReport, type WhtForm } from '@/features/finance/tax'
import { Pp30Print, VatReportPrint, WhtFormPrint } from '@/features/finance/TaxForms'
import { loadLetterhead } from '@/features/sales/pages'
import { PrintToolbar } from '@/features/sales/PrintToolbar'
import { MessagePanel } from '@/features/shell'
import { errorMessage } from '@/lib/http'
import { backendGet } from '@/lib/server/backend'
import { getCompanyContext } from '@/lib/server/company'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.finance.taxFilings.pageTitle }

type Props = { params: Promise<{ companyId: string; form: string; month: string }>; searchParams: Promise<SearchParams> }

/**
 * พิมพ์แบบของเดือน
 * - ภ.ง.ด.3 / 53: ใบแนบ (A4 แนวนอน)
 * - ภ.พ.30: แบบ (A4 แนวตั้ง); ?report=output | input = รายงานภาษีขาย / ภาษีซื้อ (A4 แนวนอน)
 */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, form, month } = await params
  const sp = await searchParams
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  if (!isTaxForm(form) || !isMonth(month)) return <MessagePanel title={texts.common.notFound} />
  const letterhead = await loadLetterhead(companyId)
  if (!letterhead) return <Alert tone="error">{texts.sales.print.unavailable}</Alert>
  const back = taxFormPage(companyId, form, month)
  const report = param(sp, 'report')
  const load = async <T,>(path: string, q: Record<string, string>) =>
    backendGet<T>(path, new URLSearchParams(q)).then(
      (r) => ({ data: r.data }),
      (err: unknown) => ({ error: errorMessage(err) }),
    )

  if (isWithholdingForm(form)) {
    const res = await load<WhtForm>(taxApi.withholding(companyId), { form, month })
    if ('error' in res) return <Alert tone="error">{res.error}</Alert>
    return (
      <>
        <PrintToolbar backHref={back} />
        <WhtFormPrint letterhead={letterhead} data={res.data} />
      </>
    )
  }
  if (report === 'output' || report === 'input') {
    const res = await load<VatReport>(taxApi.vatReport(companyId), { kind: report, month })
    if ('error' in res) return <Alert tone="error">{res.error}</Alert>
    return (
      <>
        <PrintToolbar backHref={back} />
        <VatReportPrint letterhead={letterhead} data={res.data} />
      </>
    )
  }
  const res = await load<Pp30>(taxApi.pp30(companyId), { month })
  if ('error' in res) return <Alert tone="error">{res.error}</Alert>
  return (
    <>
      <PrintToolbar backHref={back} />
      <Pp30Print letterhead={letterhead} data={res.data} />
    </>
  )
}
