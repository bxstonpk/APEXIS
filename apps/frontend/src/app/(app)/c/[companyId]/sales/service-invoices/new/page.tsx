import type { Metadata } from 'next'
import { texts } from '@/config'
import { SaleNewPage } from '@/features/sales/salePages'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.serviceInvoices.newTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  return <SaleNewPage companyId={companyId} kind="service_invoice" orderId={param(await searchParams, 'order_id')} />
}
