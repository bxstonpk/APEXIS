import type { Metadata } from 'next'
import { texts } from '@/config'
import { SaleListPage } from '@/features/sales/salePages'
import type { SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.serviceInvoices.pageTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

/** ใบแจ้งหนี้ค่าบริการ — ภาษีขายยังไม่ถึงกำหนดจนรับเงิน (การเงิน) หรือออกใบกำกับภาษีก่อน */
export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  return <SaleListPage companyId={companyId} sp={await searchParams} kind="service_invoice" />
}
