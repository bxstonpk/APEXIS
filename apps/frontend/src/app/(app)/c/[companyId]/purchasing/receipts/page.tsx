import type { Metadata } from 'next'
import { texts } from '@/config'
import { ReceiptList } from '@/features/purchasing/pages'
import { getCompanyContext } from '@/lib/server/company'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.purchasing.receipts.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ใบรับสินค้า — สร้างจากหน้าใบสั่งซื้อที่อนุมัติแล้ว (ปุ่ม "รับสินค้า") */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  const ctx = await getCompanyContext(companyId)
  if (!ctx) return null
  return <ReceiptList companyId={companyId} sp={await searchParams} />
}
