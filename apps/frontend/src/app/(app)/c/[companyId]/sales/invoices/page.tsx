import type { Metadata } from 'next'
import { texts } from '@/config'
import { SaleListPage } from '@/features/sales/salePages'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.invoices.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ขายสินค้า (ใบส่งของ / ใบแจ้งหนี้ / ใบกำกับภาษี) — สร้างเองหรือจากใบสั่งขาย */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  return <SaleListPage companyId={companyId} sp={await searchParams} kind="invoice" />
}
