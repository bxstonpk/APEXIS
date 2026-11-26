import type { Metadata } from 'next'
import { Alert } from '@/components'
import { texts } from '@/config'
import { financeApi, financePage, type Payment } from '@/features/finance/types'
import { WhtCertificate } from '@/features/finance/WhtCertificate'
import { loadItem } from '@/features/organization/pages'
import { loadLetterhead } from '@/features/sales/pages'
import { PrintToolbar } from '@/features/sales/PrintToolbar'
import { getCompanyContext } from '@/lib/server/company'

export const metadata: Metadata = { title: texts.finance.whtCertificate.pageTitle }

/** พิมพ์หนังสือรับรองการหักภาษี ณ ที่จ่าย (50 ทวิ) จากใบสำคัญจ่ายที่มีภาษีหัก ณ ที่จ่าย — 2 ฉบับ */
export default async function Page({ params }: { params: Promise<{ companyId: string; docId: string }> }) {
  const { companyId, docId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const [res, letterhead] = await Promise.all([loadItem<Payment>(financeApi.payments(companyId), docId, texts.finance.payments.notFound), loadLetterhead(companyId)])
  if ('panel' in res) return res.panel
  const d = res.item
  if (!letterhead || d.status !== 'posted' || !d.wht_certificate_no) return <Alert tone="error">{texts.finance.whtCertificate.unavailable}</Alert>
  return (
    <>
      <PrintToolbar backHref={`${financePage(companyId, 'payments')}/${d.id}`} />
      <WhtCertificate letterhead={letterhead} payment={d} />
    </>
  )
}
