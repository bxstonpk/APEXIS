import type { Metadata } from 'next'
import { texts } from '@/config'
import { SalePrintPage } from '@/features/sales/salePages'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.print.deliveryTaxInvoice }

type Props = { params: Promise<{ companyId: string; docId: string }>; searchParams: Promise<SearchParams> }

/** พิมพ์ขายสินค้า A4 — ใบส่งของ / ใบแจ้งหนี้ / ใบกำกับภาษี (ผู้ซื้อจาก bill_to ณ วันออก) */
export default async function Page({ params, searchParams }: Props) {
  const { companyId, docId } = await params
  return <SalePrintPage companyId={companyId} kind="invoice" docId={docId} as={param(await searchParams, 'as')} />
}
