import type { Metadata } from 'next'
import { texts } from '@/config'
import { PostedDocList } from '@/features/sales/pages'
import { salesApi, salesPage } from '@/features/sales/types'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.invoices.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ใบแจ้งหนี้ / ใบกำกับภาษี — สร้างจากหน้าใบสั่งขายที่อนุมัติแล้ว (ปุ่ม "ออกใบกำกับภาษี") */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales.invoices
  return (
    <PostedDocList
      companyId={companyId}
      sp={await searchParams}
      invoices
      path={salesPage(companyId, 'invoices')}
      apiPath={salesApi.invoices(companyId)}
      heading={t.heading}
      description={t.description}
    />
  )
}
