import type { Metadata } from 'next'
import { texts } from '@/config'
import { PurchaseFulfilPage } from '@/features/purchasing/pages'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.purchasing.invoices.newTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  return <PurchaseFulfilPage companyId={companyId} orderId={param(await searchParams, 'order_id')} mode="invoice" />
}
