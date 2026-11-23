import type { Metadata } from 'next'
import { texts } from '@/config'
import { InvoiceList } from '@/features/purchasing/pages'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.purchasing.invoices.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ใบแจ้งหนี้ซื้อ — บันทึกจากหน้าใบสั่งซื้อที่อนุมัติแล้ว (ปุ่ม "บันทึกใบแจ้งหนี้") */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  return <InvoiceList companyId={companyId} sp={await searchParams} />
}
