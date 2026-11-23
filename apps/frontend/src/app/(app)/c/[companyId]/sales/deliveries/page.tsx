import type { Metadata } from 'next'
import { texts } from '@/config'
import { PostedDocList } from '@/features/sales/pages'
import { salesApi, salesPage } from '@/features/sales/types'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.deliveries.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ใบส่งของ — สร้างจากหน้าใบสั่งขายที่อนุมัติแล้ว (ปุ่ม "ส่งของ") */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  const t = texts.sales.deliveries
  return (
    <PostedDocList
      companyId={companyId}
      sp={await searchParams}
      invoices={false}
      path={salesPage(companyId, 'deliveries')}
      apiPath={salesApi.deliveries(companyId)}
      heading={t.heading}
      description={t.description}
    />
  )
}
