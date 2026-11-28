import type { Metadata } from 'next'
import { texts } from '@/config'
import { SalePrintPage } from '@/features/sales/salePages'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.print.invoice }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

/** พิมพ์ใบแจ้งหนี้ค่าบริการ A4 (ไม่ใช่ใบกำกับภาษี) — ?as=tax-invoice = ใบกำกับภาษีที่ออกก่อนรับเงิน */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  return <SalePrintPage companyId={companyId} kind="service_invoice" docId={docId} as={param(await searchParams, 'as')} />
}
