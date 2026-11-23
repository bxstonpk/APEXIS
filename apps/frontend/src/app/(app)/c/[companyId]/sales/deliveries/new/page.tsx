import type { Metadata } from 'next'
import { texts } from '@/config'
import { FulfilPage } from '@/features/sales/FulfilPage'
import { param, type SearchParams } from '@/lib/url'

export const metadata: Metadata = { title: texts.sales.deliveries.newTitle }

type Props = { params: Promise<{ companyId: string }>; searchParams: Promise<SearchParams> }

export default async function Page({ params, searchParams }: Props) {
  const { companyId } = await params
  return <FulfilPage companyId={companyId} orderId={param(await searchParams, 'order_id')} mode="delivery" />
}
